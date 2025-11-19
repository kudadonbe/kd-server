# KD-Server Client UI - Build Guide

**Purpose**: Copy this guide to your new client UI repository to get started building a web interface for KD-Server.

---

## Project Setup

### Technology Stack (Recommended)

**Option 1: Simple HTML/CSS/JavaScript**
```bash
mkdir kd-client-ui
cd kd-client-ui
# Create index.html, app.js, styles.css
# No build process needed - just open in browser
```

**Option 2: React + Vite (Modern SPA)**
```bash
npm create vite@latest kd-client-ui -- --template react
cd kd-client-ui
npm install
npm install axios  # For API calls
npm run dev
```

**Option 3: Next.js (Full-stack with SSR)**
```bash
npx create-next-app@latest kd-client-ui
cd kd-client-ui
npm install axios
npm run dev
```

---

## Configuration

### Environment Variables

Create `.env.local` (never commit this file):

```env
VITE_KD_API_URL=http://localhost:8080/v1
VITE_KD_TENANT=sumeyku-istoru
VITE_KD_API_KEY=your_api_key_here
```

Or for Next.js:
```env
NEXT_PUBLIC_KD_API_URL=http://localhost:8080/v1
NEXT_PUBLIC_KD_TENANT=sumeyku-istoru
NEXT_PUBLIC_KD_API_KEY=your_api_key_here
```

### .gitignore

Add these entries:
```
.env.local
.env
*.env
node_modules/
dist/
.next/
```

---

## Core Implementation

### 1. API Client Setup

Create `src/api/kdClient.js`:

```javascript
import axios from 'axios';

const KD_API_URL = import.meta.env.VITE_KD_API_URL || 'http://localhost:8080/v1';
const KD_TENANT = import.meta.env.VITE_KD_TENANT;
const KD_API_KEY = import.meta.env.VITE_KD_API_KEY;

// Create axios instance with default config
const kdClient = axios.create({
  baseURL: KD_API_URL,
  headers: {
    'Content-Type': 'application/json',
    'X-KD-Tenant': KD_TENANT,
    'Authorization': `Bearer ${KD_API_KEY}`
  }
});

// API methods
export const kdApi = {
  // Health check
  async healthCheck() {
    const response = await axios.get(`${KD_API_URL}/healthz`);
    return response.data;
  },

  // Ingest records
  async ingest(source, records) {
    const response = await kdClient.post('/ingest', {
      source: {
        slug: source.slug,
        name: source.name
      },
      records
    });
    return response.data;
  },

  // Resolve pending records
  async resolve(batchSize = 100) {
    const response = await kdClient.post('/resolve', {
      batch_size: batchSize
    });
    return response.data;
  },

  // Lookup by phone
  async lookupByPhone(phoneNumber) {
    const response = await kdClient.get(`/lookup/phone/${encodeURIComponent(phoneNumber)}`);
    return response.data;
  },

  // Lookup by email
  async lookupByEmail(email) {
    const response = await kdClient.get(`/lookup/email/${encodeURIComponent(email)}`);
    return response.data;
  },

  // Get review queue
  async getReviewQueue(status = 'needs_review') {
    const response = await kdClient.get('/review', {
      params: { status }
    });
    return response.data;
  },

  // Submit review decision
  async submitReviewDecision(recordId, decision) {
    const response = await kdClient.post(`/review/${recordId}/decision`, {
      decision // 'accept' or 'reject'
    });
    return response.data;
  }
};

export default kdClient;
```

---

### 2. Sample Components

#### Ingest Form Component

```javascript
// IngestForm.jsx
import { useState } from 'react';
import { kdApi } from '../api/kdClient';

export default function IngestForm() {
  const [records, setRecords] = useState('');
  const [sourceName, setSourceName] = useState('sumeyku-istoru');
  const [result, setResult] = useState(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);

    try {
      const recordsArray = JSON.parse(records);
      const response = await kdApi.ingest(
        { slug: sourceName, name: sourceName },
        recordsArray
      );
      setResult(response);
    } catch (error) {
      alert('Error: ' + error.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="ingest-form">
      <h2>Ingest Records</h2>
      <form onSubmit={handleSubmit}>
        <div>
          <label>Source Name:</label>
          <input
            type="text"
            value={sourceName}
            onChange={(e) => setSourceName(e.target.value)}
            required
          />
        </div>

        <div>
          <label>Records (JSON array):</label>
          <textarea
            rows="10"
            value={records}
            onChange={(e) => setRecords(e.target.value)}
            placeholder={`[
  {
    "id": "cust001",
    "name": "John Doe",
    "email": "john@example.com",
    "phone": "+1234567890",
    "national_id": "ABC123"
  }
]`}
            required
          />
        </div>

        <button type="submit" disabled={loading}>
          {loading ? 'Ingesting...' : 'Ingest Records'}
        </button>
      </form>

      {result && (
        <div className="result">
          <h3>Ingest Result:</h3>
          <p>Created: {result.created}</p>
          <p>Linked: {result.linked}</p>
          <p>Needs Review: {result.needs_review}</p>
        </div>
      )}
    </div>
  );
}
```

#### Lookup Component

```javascript
// Lookup.jsx
import { useState } from 'react';
import { kdApi } from '../api/kdClient';

export default function Lookup() {
  const [searchType, setSearchType] = useState('email');
  const [searchValue, setSearchValue] = useState('');
  const [person, setPerson] = useState(null);
  const [loading, setLoading] = useState(false);

  const handleSearch = async (e) => {
    e.preventDefault();
    setLoading(true);
    setPerson(null);

    try {
      const result = searchType === 'email'
        ? await kdApi.lookupByEmail(searchValue)
        : await kdApi.lookupByPhone(searchValue);

      setPerson(result);
    } catch (error) {
      if (error.response?.status === 404) {
        alert('Person not found');
      } else {
        alert('Error: ' + error.message);
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="lookup">
      <h2>Lookup Person</h2>

      <form onSubmit={handleSearch}>
        <div>
          <label>Search By:</label>
          <select value={searchType} onChange={(e) => setSearchType(e.target.value)}>
            <option value="email">Email</option>
            <option value="phone">Phone (E.164 format)</option>
          </select>
        </div>

        <div>
          <label>Search Value:</label>
          <input
            type="text"
            value={searchValue}
            onChange={(e) => setSearchValue(e.target.value)}
            placeholder={searchType === 'email' ? 'user@example.com' : '+1234567890'}
            required
          />
        </div>

        <button type="submit" disabled={loading}>
          {loading ? 'Searching...' : 'Search'}
        </button>
      </form>

      {person && (
        <div className="person-details">
          <h3>Person Found</h3>
          <div className="person-info">
            <p><strong>Person ID:</strong> {person.person.personId}</p>
            <p><strong>National ID:</strong> {person.person.nationalId || 'N/A'}</p>
            <p><strong>Email:</strong> {person.person.primaryEmail || 'N/A'}</p>
            <p><strong>Phone:</strong> {person.person.primaryPhone || 'N/A'}</p>
          </div>

          <h4>Source Links ({person.links.length})</h4>
          <div className="links">
            {person.links.map((link, idx) => (
              <div key={idx} className="link-card">
                <p><strong>Source:</strong> {link.source}</p>
                <p><strong>External ID:</strong> {link.externalId}</p>
                <pre>{JSON.stringify(link.payload, null, 2)}</pre>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
```

#### Resolve Component

```javascript
// Resolve.jsx
import { useState } from 'react';
import { kdApi } from '../api/kdClient';

export default function Resolve() {
  const [batchSize, setBatchSize] = useState(100);
  const [result, setResult] = useState(null);
  const [loading, setLoading] = useState(false);

  const handleResolve = async () => {
    setLoading(true);
    try {
      const response = await kdApi.resolve(batchSize);
      setResult(response);
    } catch (error) {
      alert('Error: ' + error.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="resolve">
      <h2>Resolve Pending Records</h2>

      <div>
        <label>Batch Size:</label>
        <input
          type="number"
          value={batchSize}
          onChange={(e) => setBatchSize(parseInt(e.target.value))}
          min="1"
          max="1000"
        />
      </div>

      <button onClick={handleResolve} disabled={loading}>
        {loading ? 'Resolving...' : 'Run Resolve'}
      </button>

      {result && (
        <div className="result">
          <h3>Resolve Result:</h3>
          <p>Resolved: {result.resolved}</p>
          <p>Needs Review: {result.needs_review}</p>
          <p>Skipped: {result.skipped}</p>
        </div>
      )}
    </div>
  );
}
```

---

## Main App Structure

### Simple React App.jsx

```javascript
import { useState } from 'react';
import IngestForm from './components/IngestForm';
import Lookup from './components/Lookup';
import Resolve from './components/Resolve';
import './App.css';

function App() {
  const [activeTab, setActiveTab] = useState('ingest');

  return (
    <div className="app">
      <header>
        <h1>KD-Server Client - Sumeyku Istoru</h1>
        <nav>
          <button onClick={() => setActiveTab('ingest')}>Ingest</button>
          <button onClick={() => setActiveTab('resolve')}>Resolve</button>
          <button onClick={() => setActiveTab('lookup')}>Lookup</button>
        </nav>
      </header>

      <main>
        {activeTab === 'ingest' && <IngestForm />}
        {activeTab === 'resolve' && <Resolve />}
        {activeTab === 'lookup' && <Lookup />}
      </main>
    </div>
  );
}

export default App;
```

---

## Deployment

### Development
```bash
npm run dev
# Open http://localhost:5173
```

### Production Build
```bash
npm run build
# Deploy dist/ folder to hosting (Vercel, Netlify, etc.)
```

### Environment Variables in Production
Set these in your hosting platform:
- `VITE_KD_API_URL` - Your production KD-Server URL
- `VITE_KD_TENANT` - Your tenant slug
- `VITE_KD_API_KEY` - Your production API key

---

## Security Notes

1. **Never commit API keys** - Use environment variables
2. **CORS Configuration** - Your KD-Server may need CORS headers for browser requests
3. **HTTPS in Production** - Always use HTTPS for API calls in production
4. **API Key Rotation** - Plan for key rotation mechanism
5. **Rate Limiting** - Implement client-side throttling for API calls

---

## Next Steps

1. Copy this guide to your new repository
2. Get your tenant credentials from KD-Server admin TUI (`make tui`)
3. Set up environment variables
4. Start with Ingest and Lookup components
5. Add Review Queue interface for manual decisions
6. Enhance with dashboard and statistics

---

## Testing Your Integration

Sample test flow:
```javascript
// 1. Ingest some records
await kdApi.ingest(
  { slug: 'test', name: 'Test Import' },
  [
    {
      id: 'test001',
      name: 'Test User',
      email: 'test@example.com',
      phone: '+1234567890'
    }
  ]
);

// 2. Resolve them
await kdApi.resolve(100);

// 3. Lookup the person
const person = await kdApi.lookupByEmail('test@example.com');
console.log(person);
```
