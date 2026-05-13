# DCV GPU base layer — Amazon Linux 2 + NICE DCV + NVIDIA drivers
# Built once per driver generation; all GPU app AMIs use this as their base.
# Supports g4dn/g5 (driver 525) and g6/g6e (driver 535) via the driver_version variable.
#
# Prerequisites:
#   1. Accept the AWS NICE DCV AMI Marketplace subscription (GPU variant):
#      https://aws.amazon.com/marketplace/pp/prodview-ygflzfzntbqqs
#   2. See base/nvidia-versions.yaml for the tested driver matrix.
#
# Usage:
#   # g6/g6e — L4/L40S (535 series, CUDA 12.2):
#   packer build \
#     -var "region=us-east-1" \
#     -var "instance_type=g6.xlarge" \
#     -var "driver_version=535.129.03" \
#     -var "cuda_version=12.2" \
#     -var "layer_name=dcv-gpu-al2-535" \
#     base/dcv-gpu-al2.pkr.hcl
#
#   # g4dn/g5 — T4/A10G (525 series, CUDA 12.0):
#   packer build \
#     -var "region=us-east-1" \
#     -var "instance_type=g5.xlarge" \
#     -var "driver_version=525.105.17" \
#     -var "cuda_version=12.0" \
#     -var "layer_name=dcv-gpu-al2-525" \
#     base/dcv-gpu-al2.pkr.hcl

packer {
  required_plugins {
    amazon = {
      version = ">= 1.3.0"
      source  = "github.com/hashicorp/amazon"
    }
  }
}

variable "region" {
  type    = string
  default = "us-east-1"
}

variable "instance_type" {
  type    = string
  default = "g6.xlarge"
}

variable "driver_version" {
  type    = string
  default = "535.129.03"
}

variable "cuda_version" {
  type    = string
  default = "12.2"
}

variable "layer_name" {
  type    = string
  default = "dcv-gpu-al2-535"
}

# AWS NICE DCV for Amazon Linux 2 (GPU variant) from Marketplace.
data "amazon-ami" "dcv-gpu-base" {
  filters = {
    name                = "DCV-AmazonLinux2-*-NVIDIA*"
    root-device-type    = "ebs"
    virtualization-type = "hvm"
    architecture        = "x86_64"
  }
  most_recent = true
  owners      = ["679593333241"]
  region      = var.region
}

source "amazon-ebs" "dcv-gpu-al2" {
  region        = var.region
  source_ami    = data.amazon-ami.dcv-gpu-base.id
  instance_type = var.instance_type
  ssh_username  = "ec2-user"

  ami_name        = "spore-${var.layer_name}-{{timestamp}}"
  ami_description = "spore.host DCV GPU base — AL2 + NICE DCV + NVIDIA ${var.driver_version} / CUDA ${var.cuda_version}"

  tags = {
    "spore:layer"          = var.layer_name
    "spore:nvidia-driver"  = var.driver_version
    "spore:cuda"           = var.cuda_version
    "spore:build-date"     = "{{timestamp}}"
    "spore:managed"        = "true"
  }

  launch_block_device_mappings {
    device_name           = "/dev/xvda"
    volume_size           = 50
    volume_type           = "gp3"
    delete_on_termination = true
  }
}

build {
  name    = "dcv-gpu-al2"
  sources = ["source.amazon-ebs.dcv-gpu-al2"]

  # Verify NVIDIA driver is functional (pre-installed by Marketplace AMI)
  provisioner "shell" {
    inline = [
      "nvidia-smi --query-gpu=name,driver_version,memory.total --format=csv,noheader",
    ]
  }

  # Install spored
  provisioner "shell" {
    inline = [
      "curl -fsSL https://spawn-binaries-us-east-1.s3.amazonaws.com/spored-linux-amd64 -o /tmp/spored",
      "chmod +x /tmp/spored",
      "sudo mv /tmp/spored /usr/local/bin/spored",
    ]
  }

  # Configure DCV for application streaming (virtual session, GPU-accelerated)
  provisioner "shell" {
    inline = [
      "sudo sed -i 's/#create-session = true/create-session = true/' /etc/dcv/dcv.conf",
      # Enable GPU rendering in DCV
      "sudo sed -i 's/#gl-displays/gl-displays/' /etc/dcv/dcv.conf || true",
      "sudo systemctl enable dcvserver",
    ]
  }

  post-processor "manifest" {
    output     = "packer-manifest-${var.layer_name}.json"
    strip_path = true
  }
}
