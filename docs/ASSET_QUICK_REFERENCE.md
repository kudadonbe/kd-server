# Asset Management Quick Reference

## Asset Number Format
```
{office}-{year}-{catNo}-{typeNumber}-{serialIncrement}
Example: B15-2024-02-073-0001
```

## Main Categories (Level 1)

| Code | Name |
|------|------|
| `01` | Land and Building |
| `02` | Furniture, Fixtures and Fittings |
| `03` | Equipment |
| `04` | Vehicular Equipment and Vehicles |
| `05` | Tools, Instrument and Apparatus |
| `06` | Copy Rights and Patents |
| `07` | Heritage Assets |
| `08` | Lagoons and Islands |
| `09` | Reference Books & Exhibition |
| `290` | Non Asset Inventory |

## Common Subcategories (Level 2)

### Category 01 - Land and Building
- `10` Land
- `11` Residential Buildings
- `12` Non Residential Buildings

### Category 02 - Furniture
- `13` College Furniture
- `14` Hospital Furniture
- `15` Office Furniture
- `16` School Furniture
- `17` Household Furniture

### Category 03 - Equipment
- `19` Hospital Equipment
- `20` Laboratory Equipment
- `21` Office Equipment
- `22` Communication Infrastructure
- `23` IT-Related Hardware

### Category 04 - Vehicles
- `24` Vehicular Equipment (excavators, cranes)
- `25` Motor Vehicles (cars, trucks, buses)
- `26` Ships and Boats
- `27` Other Vessels
- `28` Aerospace Equipment

### Category 05 - Tools
- `29` Apparatus
- `30` Instruments
- `31` Tools

## Frequently Used Type Codes (Level 3)

### Office Furniture (Sub 15)
- `079` Executive Chair
- `089` Executive Table
- `090` Conference Table
- `097` Shelves/Cupboards

### IT Hardware (Sub 23)
- `150` Computers
- `151` Printers
- `152` Projectors
- `155` Servers
- `156` Switches

### Motor Vehicles (Sub 25)
- `198` Buses
- `199` Cars
- `203` Pick up
- `204` Trucks
- `205` Vans

### College Furniture (Sub 13)
- `073` Computer Table
- `074` Desk/Table
- `075` Chair
- `076` Bookshelves

## API Endpoints

```bash
# Ingest assets
POST /v1/ingest/assets
{
  "source": {"slug": "neelan", "name": "NeelanPortal"},
  "records": [...]
}

# Lookup by asset number
GET /v1/lookup/asset/B15-2024-02-073-0001

# Query assets
GET /v1/assets?office=B15&categoryNo=02

# Get classification hierarchy
GET /v1/assets/categories
GET /v1/assets/categories/02/types
```

## Minimum Required Fields

```json
{
  "office": "B15",
  "categoryNo": "02",
  "typeNumber": "073",
  "description1": "Computer Table",
  "originalValue": 5000.00,
  "acquisitionDate": "2024-01-15",
  "location": "Lab A",
  "quantity": 1
}
```

## Office Codes (Examples)
- `MLE` - Male' City
- `ADU` - Addu City
- `FML` - Fuvahmulah
- `B15` - Business Unit 15
- `HDH` - Haa Dhaalu Atoll

## Validation Rules
- Office: 2-4 alphanumeric characters
- Year: YYYY (not future, max 100 years past)
- Category: 01-09 or 290
- Type: 3-digit code from hierarchy
- Serial: Auto-generated (0001-9999)
- Value: Must be positive number

## Asset Statuses
- `active` - In use
- `disposed` - Removed from service
- `under_maintenance` - Temporarily unavailable
- `transferred` - Moved to another location
- `stolen` - Lost/stolen
- `damaged` - Requires repair

---

For complete specification, see `ASSET_MANAGEMENT_SPEC.md`
