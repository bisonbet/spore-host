# DCV CPU base layer — Amazon Linux 2 + NICE DCV (no GPU)
# Built once; all CPU-only app AMIs (IGV, QGIS, Fiji, DS9, etc.) use this as their base.
#
# Prerequisites:
#   1. Accept the AWS NICE DCV AMI Marketplace subscription:
#      https://aws.amazon.com/marketplace/pp/prodview-ygflzfzntbqqs (AL2 variant)
#   2. Set SPORE_BUILD_ACCOUNT env var to your infra AWS account ID (812107987990)
#   3. Install Packer: https://developer.hashicorp.com/packer/install
#
# Usage:
#   packer build -var "region=us-east-1" base/dcv-cpu-al2.pkr.hcl

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
  default = "c7i.xlarge"
}

# AWS NICE DCV for Amazon Linux 2 (CPU variant) — source AMI from Marketplace.
# Find the current AMI ID for your region at:
#   aws ec2 describe-images --owners 679593333241 \
#     --filters "Name=name,Values=DCV-AmazonLinux2-*" \
#     --query "sort_by(Images, &CreationDate)[-1].ImageId" \
#     --region us-east-1
data "amazon-ami" "dcv-cpu-base" {
  filters = {
    name                = "DCV-AmazonLinux2-*"
    root-device-type    = "ebs"
    virtualization-type = "hvm"
    architecture        = "x86_64"
  }
  most_recent = true
  owners      = ["679593333241"] # AWS Marketplace AMI owner for NICE DCV
  region      = var.region
}

source "amazon-ebs" "dcv-cpu-al2" {
  region        = var.region
  source_ami    = data.amazon-ami.dcv-cpu-base.id
  instance_type = var.instance_type
  ssh_username  = "ec2-user"

  ami_name        = "spore-dcv-cpu-al2-{{timestamp}}"
  ami_description = "spore.host DCV CPU base layer — AL2 + NICE DCV, no GPU"

  tags = {
    "spore:layer"      = "dcv-cpu-al2"
    "spore:build-date" = "{{timestamp}}"
    "spore:managed"    = "true"
  }

  launch_block_device_mappings {
    device_name           = "/dev/xvda"
    volume_size           = 30
    volume_type           = "gp3"
    delete_on_termination = true
  }
}

build {
  name    = "dcv-cpu-al2"
  sources = ["source.amazon-ebs.dcv-cpu-al2"]

  # Install spored (spore.host instance lifecycle daemon)
  provisioner "shell" {
    inline = [
      "curl -fsSL https://spawn-binaries-us-east-1.s3.amazonaws.com/spored-linux-amd64 -o /tmp/spored",
      "chmod +x /tmp/spored",
      "sudo mv /tmp/spored /usr/local/bin/spored",
      "sudo /usr/local/bin/spored --version",
    ]
  }

  # Ensure DCV is configured for application streaming mode (no desktop manager).
  provisioner "shell" {
    inline = [
      # Set DCV to virtual session mode (application streaming — no physical display)
      "sudo sed -i 's/#create-session = true/create-session = true/' /etc/dcv/dcv.conf",
      "sudo systemctl enable dcvserver",
    ]
  }

  post-processor "manifest" {
    output     = "packer-manifest-dcv-cpu-al2.json"
    strip_path = true
  }
}
