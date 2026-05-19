# Python Bindings

The `truffle` Python package wraps the truffle Go library via native cgo bindings. It's 10–50x faster than subprocess-based wrappers because it calls the Go shared library directly rather than spawning a child process.

::: tip Not on PyPI yet
Install directly from the repository until the package is published:
```sh
pip install "git+https://github.com/spore-host/spore-host.git#subdirectory=truffle/bindings/python"
```
Or from a local clone:
```sh
pip install ./truffle/bindings/python
```
:::

## Requirements

- Python 3.8+
- Go 1.26+ compiler (required at install time — the build compiles `libtruffle.so`/`.dylib`)
- AWS credentials configured (same credential chain as the CLI)

Install Go if needed:

::: code-group
```sh [macOS]
brew install go
```
```sh [Ubuntu/Debian]
sudo apt install golang-go
```
:::

## Authentication

The bindings use the same AWS credential chain as the truffle CLI — `~/.aws/credentials`, environment variables, or instance metadata. No separate configuration is needed.

```python
from truffle import Truffle

tf = Truffle()  # uses ambient AWS credentials automatically
```

To use a specific profile or region:

```python
import os
os.environ["AWS_PROFILE"] = "my-research-account"
os.environ["AWS_DEFAULT_REGION"] = "us-east-1"

tf = Truffle()
```

## Quick start

```python
from truffle import Truffle

tf = Truffle()

# Search instance types by pattern
results = tf.search("m7i.large", regions=["us-east-1"])
for r in results:
    print(f"{r.instance_type}: {r.vcpus} vCPUs, {r.memory_gib} GiB")

# Get Spot pricing
spots = tf.spot("m8g.*", max_price=0.20, sort_by_price=True)
for s in spots[:5]:
    print(f"{s.instance_type}: ${s.spot_price:.4f}/hr, saves {s.savings_percent:.0f}%")

# Check GPU capacity reservations
from truffle import ReservationType
capacity = tf.capacity(gpu_only=True, available_only=True,
                       reservation_type=ReservationType.ODCR)
for c in capacity:
    print(f"{c.instance_type}: {c.available_capacity}/{c.total_capacity} available")
```

---

## API reference

### `Truffle(lib_path=None)`

Creates a client. `lib_path` is the path to the compiled shared library. If omitted, it is auto-detected from the package directory.

```python
tf = Truffle()
tf = Truffle(lib_path="/usr/local/lib/libtruffle.so")
```

---

### `search(pattern, regions=None, architecture=None, min_vcpus=None, min_memory=None, include_azs=True)`

Search for instance types by wildcard pattern.

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `pattern` | str | (required) | Wildcard pattern (e.g., `"m7i.large"`, `"m8g.*"`, `"c[6-8]*"`) |
| `regions` | list[str] | (all enabled) | Regions to search |
| `architecture` | str | (any) | `"x86_64"`, `"arm64"`, or `"i386"` |
| `min_vcpus` | int | (any) | Minimum vCPU count |
| `min_memory` | float | (any) | Minimum memory in GiB |
| `include_azs` | bool | `True` | Include AZ list in results |

**Returns:** `list[InstanceType]`

**Example:**
```python
results = tf.search(
    "m8g.*",
    architecture="arm64",
    min_vcpus=8,
    min_memory=32,
    regions=["us-east-1", "us-west-2"]
)
```

---

### `az(pattern, regions=None, min_az_count=None, azs=None)`

Search with an availability zone-first perspective. Results are sorted by AZ count (most available AZs first).

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `pattern` | str | (required) | Wildcard pattern |
| `regions` | list[str] | (all enabled) | Regions to search |
| `min_az_count` | int | (any) | Minimum AZs required per region |
| `azs` | list[str] | (all) | Filter to specific AZs (e.g., `["us-east-1a", "us-east-1b"]`) |

**Returns:** `list[InstanceType]`

**Example:**
```python
# Find m7i.large available in 3+ AZs for HA deployment
results = tf.az("m7i.large", min_az_count=3)
for r in results:
    print(f"{r.region}: {len(r.availability_zones)} AZs — {r.availability_zones}")
```

---

### `spot(pattern, regions=None, max_price=None, show_savings=False, sort_by_price=False)`

Get current Spot prices for matching instance types.

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `pattern` | str | (required) | Wildcard pattern |
| `regions` | list[str] | (all enabled) | Regions to search |
| `max_price` | float | (any) | Maximum acceptable price per hour (USD) |
| `show_savings` | bool | `False` | Populate `savings_percent` field |
| `sort_by_price` | bool | `False` | Sort results cheapest first |

**Returns:** `list[SpotPrice]`

**Example:**
```python
spots = tf.spot("c8g.*", max_price=0.15, sort_by_price=True, show_savings=True)

cheapest = spots[0]
print(f"Best: {cheapest.instance_type} in {cheapest.availability_zone}")
print(f"  Price: ${cheapest.spot_price:.4f}/hr")
print(f"  Saves: {cheapest.savings_percent:.0f}% vs on-demand")
```

---

### `capacity(instance_types=None, regions=None, reservation_type=ReservationType.ODCR, gpu_only=False, available_only=False, min_capacity=None)`

Query On-Demand Capacity Reservations (ODCRs) or Capacity Blocks for ML.

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `instance_types` | list[str] | (all) | Filter by specific instance types |
| `regions` | list[str] | (all enabled) | Regions to search |
| `reservation_type` | ReservationType | `ODCR` | `ReservationType.ODCR` or `ReservationType.CAPACITY_BLOCKS` |
| `gpu_only` | bool | `False` | Only return GPU/ML instance families (p, g, inf, trn) |
| `available_only` | bool | `False` | Only return reservations with available capacity |
| `min_capacity` | int | (any) | Minimum available capacity |

**Returns:** `list[CapacityReservation]` (ODCR) or `list[CapacityBlock]` (Capacity Blocks)

**Capacity Blocks** are fixed-duration reservations for high-performance ML training (p5, p4d, trn1). **ODCRs** are continuous reservations with no fixed end date, used for inference and development.

**Example:**
```python
from truffle import ReservationType

# Check training capacity blocks
blocks = tf.capacity(
    instance_types=["p5.48xlarge"],
    reservation_type=ReservationType.CAPACITY_BLOCKS,
    regions=["us-east-1"]
)
for block in blocks:
    print(f"{block.instance_count}x {block.instance_type}")
    print(f"  Period: {block.start_date} to {block.end_date}")
    print(f"  UltraCluster: {block.ultra_cluster_placement}")

# Check inference ODCRs
odcrs = tf.capacity(
    instance_types=["inf2.xlarge"],
    reservation_type=ReservationType.ODCR,
    available_only=True
)
for c in odcrs:
    print(f"{c.instance_type}: {c.available_capacity} available in {c.availability_zone}")
```

---

### `list_regions()`

Returns the list of enabled AWS regions for your account.

**Returns:** `list[str]`

---

## Data classes

### `InstanceType`

```python
@dataclass
class InstanceType:
    instance_type: str
    region: str
    vcpus: int
    memory_gib: float
    architecture: str
    availability_zones: Optional[List[str]]

    @property
    def memory_mib(self) -> float: ...  # convenience: memory_gib * 1024
```

### `SpotPrice`

```python
@dataclass
class SpotPrice:
    instance_type: str
    region: str
    availability_zone: str
    spot_price: float
    on_demand_price: Optional[float]
    savings_percent: Optional[float]
    timestamp: Optional[str]
```

### `CapacityReservation`

Returned for `reservation_type=ReservationType.ODCR`.

```python
@dataclass
class CapacityReservation:
    reservation_id: str
    instance_type: str
    region: str
    availability_zone: str
    total_capacity: int
    available_capacity: int
    used_capacity: int
    state: str
    platform: Optional[str]
    end_date: Optional[str]
```

### `CapacityBlock`

Returned for `reservation_type=ReservationType.CAPACITY_BLOCKS`.

```python
@dataclass
class CapacityBlock:
    capacity_block_id: str
    instance_type: str
    instance_count: int
    availability_zone: str
    start_date: str
    end_date: str
    duration_hours: int
    state: str
    ultra_cluster_placement: bool
```

### `ReservationType`

```python
class ReservationType(Enum):
    ODCR = 0              # On-Demand Capacity Reservations (continuous)
    CAPACITY_BLOCKS = 1   # Capacity Blocks for ML (fixed-duration training)
```

---

## Convenience functions

Module-level shortcuts that create a default `Truffle()` client on the fly:

```python
from truffle import search_instances, get_spot_prices, check_gpu_capacity

instances = search_instances("m7i.*", min_vcpus=4)
spots = get_spot_prices("m8g.large", max_price=0.10)
gpu = check_gpu_capacity(available_only=True, min_capacity=1)
```

---

## Error handling

```python
from truffle import TruffleError, TruffleNotFoundError

try:
    tf = Truffle()
    results = tf.search("m7i.large")
except TruffleNotFoundError:
    print("libtruffle shared library not found — rebuild with: pip install .")
except TruffleError as e:
    print(f"Error: {e}")
```

---

## Examples

### Pre-training capacity check

```python
from truffle import Truffle, ReservationType

tf = Truffle()

def can_train(instance_type: str, count: int) -> bool:
    # Check Capacity Blocks first
    blocks = tf.capacity(
        instance_types=[instance_type],
        reservation_type=ReservationType.CAPACITY_BLOCKS
    )
    for block in blocks:
        if block.instance_count >= count and block.state == "active":
            return True

    # Fall back to ODCRs
    odcrs = tf.capacity(
        instance_types=[instance_type],
        reservation_type=ReservationType.ODCR,
        available_only=True,
        min_capacity=count
    )
    return bool(odcrs)

if can_train("p5.48xlarge", 8):
    print("Capacity available — proceed with training")
```

### Spot price optimization

```python
spots = tf.spot("c8g.*", sort_by_price=True, show_savings=True)

# Group by region to find cheapest per region
by_region: dict[str, list] = {}
for s in spots:
    by_region.setdefault(s.region, []).append(s)

for region, prices in sorted(by_region.items()):
    best = prices[0]
    print(f"{region}: {best.instance_type} @ ${best.spot_price:.4f}/hr")
```

### Multi-AZ HA planning

```python
# Find instances available in 3+ AZs across all regions
ha_instances = tf.az("m7i.large", min_az_count=3)
for r in ha_instances:
    print(f"{r.region} ({len(r.availability_zones)} AZs): {r.instance_type}")
```
