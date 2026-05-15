#!/bin/bash
set -e

# Install Python zstandard module for Zstd RPM extraction
# AL2's rpm doesn't support Zstd compression (used by nvidia-container-toolkit >= 1.14)
pip3 install --quiet zstandard

# Write Python extraction script
cat > /tmp/extract-nctk.py << 'PYEOF'
#!/usr/bin/env python3
"""Extract Zstd-compressed RPMs for NVIDIA Container Toolkit on AL2."""
import zstandard, subprocess, os, sys

def extract_rpm_zstd(rpm_path, dest='/'):
    with open(rpm_path, 'rb') as f:
        data = f.read()
    # Zstd frame magic (little-endian): 28 B5 2F FD
    zstd_magic = b'\x28\xb5\x2f\xfd'
    pos = data.find(zstd_magic)
    if pos == -1:
        print(f'  WARNING: No Zstd magic in {rpm_path}, skipping')
        return False
    payload = data[pos:]
    dctx = zstandard.ZstdDecompressor()
    decompressed = dctx.decompress(payload, max_output_size=800*1024*1024)
    proc = subprocess.run(['cpio', '-idmu', '-D', dest],
                          input=decompressed, capture_output=True)
    if proc.returncode != 0:
        lines = proc.stderr.decode(errors='replace').split('\n')[:2]
        print(f'  cpio warnings: {lines}')
    return True

for rpm in sorted(os.listdir('/tmp/nctk')):
    if not rpm.endswith('.rpm'):
        continue
    print(f'Extracting {rpm}')
    extract_rpm_zstd(f'/tmp/nctk/{rpm}')

print('Extraction complete')
PYEOF

# Run as root so cpio can write to /usr
python3 /tmp/extract-nctk.py

ldconfig

# Ensure binaries are executable and in PATH
for bin in nvidia-ctk nvidia-container-runtime nvidia-container-hook; do
    found=$(find /usr -name "$bin" -type f 2>/dev/null | head -1)
    if [ -n "$found" ]; then
        chmod +x "$found"
        echo "$bin found at $found"
    else
        echo "WARNING: $bin not found after extraction"
    fi
done

# Configure Docker to use NVIDIA runtime
mkdir -p /etc/docker
cat > /etc/docker/daemon.json << 'JSON'
{
  "default-runtime": "nvidia",
  "runtimes": {
    "nvidia": {
      "path": "/usr/bin/nvidia-container-runtime",
      "runtimeArgs": []
    }
  }
}
JSON

echo "NVIDIA Container Toolkit installed and Docker configured"
echo "nvidia-container-runtime: $(which nvidia-container-runtime 2>/dev/null || echo NOT FOUND)"
echo "nvidia-ctk: $(which nvidia-ctk 2>/dev/null || echo NOT FOUND)"
