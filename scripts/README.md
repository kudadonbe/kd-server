# Asset Management Scripts

This directory contains helper scripts for the KD-Server Asset Management API.

## Quick Start Script

**File:** `asset_quickstart.sh`

A demonstration script that showcases the complete asset management workflow.

### Prerequisites

1. KD-Server running at `http://localhost:8080`
2. `jq` installed (`brew install jq` on macOS)
3. API key from the TUI

### Usage

```bash
# Set your credentials
export KD_TENANT=moe
export KD_API_KEY=your_api_key_here

# Run the demo
./scripts/asset_quickstart.sh
```

### What it Does

The script demonstrates:
1. ✅ Fetching asset classification categories
2. ✅ Creating a test asset
3. ✅ Looking up the created asset
4. ✅ Transferring the asset to a new location
5. ✅ Viewing the asset history

### Example Output

```
KD-Server Asset Management - Quick Start

[1/5] Fetching categories...
   ✓ Found 10 categories
[2/5] Creating test asset...
   ✓ Created: B15-2024-02-073-0001
[3/5] Looking up asset...
   ✓ Type: Computer Table
[4/5] Transferring asset...
   ✓ Transferred
[5/5] Checking history...
   • created
   • transfer

Demo Complete! Asset: B15-2024-02-073-0001
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `API_URL` | KD-Server API endpoint | `http://localhost:8080` |
| `KD_TENANT` | Tenant slug | `moe` |
| `KD_API_KEY` | API authentication key | (required) |

## More Examples

See the full usage guide for more examples:
- [Asset Usage Guide](../docs/ASSET_USAGE_GUIDE.md)
- [API Examples](../api/ASSET_API_EXAMPLES.md)
