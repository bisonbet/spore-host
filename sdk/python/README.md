# spore-host

Python SDK for [spore.host](https://spore.host) — ephemeral EC2 compute for researchers.

```sh
pip install spore-host
```

## Quick start

```python
import spore

# Find instances
results = spore.truffle.find("amd epyc genoa", region="us-east-1")
for r in results:
    print(r.instance_type, f"${r.on_demand_price:.4f}/hr")

# Check Spot prices
prices = spore.truffle.spot("c8a.2xlarge", region="us-east-1")
cheapest = min(prices, key=lambda p: p.spot_price)

# Manage running instances
instances = spore.spawn.list()
inst = spore.spawn.status("sim-run-42")
inst.extend("2h")     # push the TTL deadline forward
inst.wait("terminated", on_status=lambda i: print(i.state))
```

Works in Jupyter notebooks, [marimo](https://marimo.io), and plain Python scripts.

## Documentation

Full documentation at **[docs.spore.host/guides/python-sdk](https://docs.spore.host/guides/python-sdk)**

- [Jupyter example notebook](examples/jupyter_example.ipynb)
- [marimo example](examples/marimo_example.py)
- [Script example](examples/script_example.py)

## Requirements

- Python 3.9+
- AWS credentials configured (`~/.aws/credentials` or environment variables)
- `boto3`, `requests`

Optional notebook extras: `pip install "spore-host[jupyter]"`

## License

Apache 2.0 — see [LICENSE](../../LICENSE)
