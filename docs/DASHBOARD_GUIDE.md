📚 **[Documentation Index](./README.md)** | [🏠 Project Home](../README.md)

---

# Dashboard Guide for Go Search Engine

This guide provides a comprehensive plan for building a web dashboard for the Go Search Engine using the provided
OpenAPI specification.

## 📋 Overview

The dashboard will provide a user-friendly interface for:

- Managing search indexes (create, list, view, delete)
- Adding and managing documents
- Performing searches with advanced filtering
- Monitoring search performance and results

## 🛠️ Recommended Tech Stack

### Frontend Framework Options

#### Option 1: React + TypeScript (Recommended)

```bash
# Create new React app with TypeScript
npx create-react-app search-engine-dashboard --template typescript
cd search-engine-dashboard

# Install dependencies
npm install @mui/material @emotion/react @emotion/styled
npm install @mui/icons-material @mui/x-data-grid
npm install axios react-router-dom
npm install @types/react-router-dom
```

#### Option 2: Vue 3 + TypeScript

```bash
# Create Vue app
npm create vue@latest search-engine-dashboard
cd search-engine-dashboard
npm install

# Install UI library
npm install vuetify
npm install axios vue-router
```

#### Option 3: Next.js (Full-stack)

```bash
npx create-next-app@latest search-engine-dashboard --typescript --tailwind --eslint
cd search-engine-dashboard
npm install @headlessui/react @heroicons/react
npm install axios swr
```

## 🎨 UI Component Library

### Material-UI (React) - Recommended

- Rich component ecosystem
- Built-in theming
- Excellent documentation
- Data grid component for results

### Tailwind CSS Alternative

- Highly customizable
- Smaller bundle size
- Modern utility-first approach

## 📱 Dashboard Structure

### 1. Main Layout

```
┌─────────────────────────────────────┐
│ Header (Logo, Navigation, User)     │
├─────────────────────────────────────┤
│ Sidebar │ Main Content Area         │
│         │                           │
│ - Home  │ Dynamic content based     │
│ - Index │ on selected route         │
│ - Docs  │                           │
│ - Search│                           │
│         │                           │
└─────────────────────────────────────┘
```

### 2. Page Structure

- **Dashboard Home**: Statistics and recent activity
- **Index Management**: CRUD operations for indexes
- **Document Management**: Upload and manage documents
- **Search Interface**: Advanced search with filters
- **Settings**: Configuration and preferences

## 🔧 Core Components to Build

### 1. Index Management Components

#### IndexList Component

```typescript
interface Index {
  name: string;
  searchable_fields: string[];
  filterable_fields: string[];
  ranking_criteria: RankingCriterion[];
  min_word_size_for_1_typo: number;
  min_word_size_for_2_typos: number;
  fields_without_prefix_search: string[];
}

interface IndexListProps {
  indexes: Index[];
  onEdit: (index: Index) => void;
  onDelete: (indexName: string) => void;
  onView: (indexName: string) => void;
}
```

#### IndexForm Component

```typescript
interface IndexFormProps {
  index?: Index; // undefined for create, populated for edit
  onSubmit: (index: Index) => void;
  onCancel: () => void;
}
```

### 2. Document Management Components

#### DocumentUpload Component

```typescript
interface DocumentUploadProps {
  indexName: string;
  onUpload: (documents: Document[]) => void;
}

// Support for:
// - Single document input
// - Bulk JSON upload
// - CSV import with field mapping
// - Drag & drop interface
```

#### DocumentList Component

```typescript
interface DocumentListProps {
  indexName: string;
  documents: SearchHit[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
  };
  onPageChange: (page: number) => void;
}
```

### 3. Search Interface Components

#### SearchForm Component

```typescript
interface SearchFormProps {
  indexName: string;
  onSearch: (query: SearchRequest) => void;
}

// Features:
// - Text input with autocomplete
// - Advanced filters panel
// - Sort/ranking options
// - Pagination controls
```

#### SearchResults Component

```typescript
interface SearchResultsProps {
  results: SearchResult;
  onDocumentClick: (document: Document) => void;
  onExport: () => void;
}

// Features:
// - Result cards/table view
// - Highlighted search terms
// - Score display
// - Field matches visualization
// - Export functionality
```

#### FilterPanel Component

```typescript
interface Filter {
  field: string;
  operator: string;
  value: any;
}

interface FilterPanelProps {
  availableFields: string[];
  filters: Filter[];
  onFiltersChange: (filters: Filter[]) => void;
}
```

## 🔌 API Integration

### 1. API Client Setup

#### Axios Configuration

```typescript
// api/client.ts
import axios from "axios";

const apiClient = axios.create({
  baseURL: "http://localhost:8080",
  headers: {
    "Content-Type": "application/json",
  },
});

// Request interceptor for logging
apiClient.interceptors.request.use((config) => {
  console.log(`API Request: ${config.method?.toUpperCase()} ${config.url}`);
  return config;
});

// Response interceptor for error handling
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    console.error("API Error:", error.response?.data || error.message);
    return Promise.reject(error);
  }
);

export default apiClient;
```

#### TypeScript API Service

```typescript
// api/searchEngineApi.ts
import apiClient from "./client";
import { Index, Document, SearchRequest, SearchResult } from "./types";

export class SearchEngineAPI {
  // Index Management
  async createIndex(index: Index): Promise<{ message: string }> {
    const response = await apiClient.post("/indexes", index);
    return response.data;
  }

  async listIndexes(): Promise<{ indexes: string[]; count: number }> {
    const response = await apiClient.get("/indexes");
    return response.data;
  }

  async getIndex(name: string): Promise<Index> {
    const response = await apiClient.get(`/indexes/${name}`);
    return response.data;
  }

  async deleteIndex(name: string): Promise<{ message: string }> {
    const response = await apiClient.delete(`/indexes/${name}`);
    return response.data;
  }

  async updateIndexSettings(name: string, settings: any): Promise<any> {
    const response = await apiClient.patch(
      `/indexes/${name}/settings`,
      settings
    );
    return response.data;
  }

  // Document Management
  async addDocuments(
    indexName: string,
    documents: Document | Document[]
  ): Promise<{ message: string }> {
    const response = await apiClient.put(
      `/indexes/${indexName}/documents`,
      documents
    );
    return response.data;
  }

  // Search
  async search(
    indexName: string,
    request: SearchRequest
  ): Promise<SearchResult> {
    const response = await apiClient.post(
      `/indexes/${indexName}/_search`,
      request
    );
    return response.data;
  }
}

export const searchAPI = new SearchEngineAPI();
```

### 2. State Management

#### React Context (Simple)

```typescript
// context/SearchEngineContext.tsx
import React, { createContext, useContext, useReducer } from "react";

interface State {
  indexes: Index[];
  currentIndex: string | null;
  searchResults: SearchResult | null;
  loading: boolean;
  error: string | null;
}

type Action =
  | { type: "SET_LOADING"; payload: boolean }
  | { type: "SET_ERROR"; payload: string | null }
  | { type: "SET_INDEXES"; payload: Index[] }
  | { type: "SET_CURRENT_INDEX"; payload: string }
  | { type: "SET_SEARCH_RESULTS"; payload: SearchResult };

const SearchEngineContext = createContext<{
  state: State;
  dispatch: React.Dispatch<Action>;
}>({} as any);

export const useSearchEngine = () => useContext(SearchEngineContext);
```

#### Redux Toolkit (Advanced)

```typescript
// store/indexSlice.ts
import { createSlice, createAsyncThunk } from "@reduxjs/toolkit";
import { searchAPI } from "../api/searchEngineApi";

export const fetchIndexes = createAsyncThunk(
  "indexes/fetchIndexes",
  async () => {
    const response = await searchAPI.listIndexes();
    return response.indexes;
  }
);

const indexSlice = createSlice({
  name: "indexes",
  initialState: {
    indexes: [],
    loading: false,
    error: null,
  },
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(fetchIndexes.pending, (state) => {
        state.loading = true;
      })
      .addCase(fetchIndexes.fulfilled, (state, action) => {
        state.loading = false;
        state.indexes = action.payload;
      })
      .addCase(fetchIndexes.rejected, (state, action) => {
        state.loading = false;
        state.error = action.error.message || "Failed to fetch indexes";
      });
  },
});

export default indexSlice.reducer;
```

## 🎯 Key Features to Implement

### 1. Index Management Dashboard

- **Create Index Wizard**: Step-by-step form for index creation
- **Index List View**: Table with actions (view, edit, delete)
- **Index Details**: Settings, statistics, document count
- **Settings Editor**: Update index configuration

### 2. Document Management

- **Upload Interface**:
  - Single document form
  - Bulk JSON upload
  - CSV import with field mapping
  - Drag & drop file upload
- **Document Browser**: Paginated list with search
- **Document Editor**: Edit individual documents
- **Validation**: Real-time validation against index schema

### 3. Advanced Search Interface

- **Query Builder**: Visual query construction
- **Filter Panel**: Dynamic filters based on filterable fields
- **Results Display**:
  - Cards or table view
  - Highlighted search terms
  - Relevance scores
  - Field matches
- **Export Options**: JSON, CSV, PDF export
- **Search History**: Save and recall searches

### 4. Analytics & Monitoring

- **Search Performance**: Response times, query volume
- **Index Statistics**: Document counts, index size
- **Popular Queries**: Most frequent searches
- **Error Tracking**: Failed searches, system errors

## 🔍 Example Implementation: Search Component

```typescript
// components/SearchInterface.tsx
import React, { useState, useEffect } from "react";
import {
	TextField,
	Button,
	Paper,
	Grid,
	Typography,
	Chip,
	CircularProgress,
} from "@mui/material";
import { searchAPI } from "../api/searchEngineApi";

interface SearchInterfaceProps {
	indexName: string;
}

export const SearchInterface: React.FC<SearchInterfaceProps> = ({
																																	indexName,
																																}) => {
	const [query, setQuery] = useState("");
	const [filters, setFilters] = useState<Record<string, any>>({});
	const [results, setResults] = useState<SearchResult | null>(null);
	const [loading, setLoading] = useState(false);
	const [page, setPage] = useState(1);

	const handleSearch = async () => {
		setLoading(true);
		try {
			const searchRequest: SearchRequest = {
				query,
				filters,
				page,
				page_size: 10,
			};

			const searchResults = await searchAPI.search(indexName, searchRequest);
			setResults(searchResults);
		} catch (error) {
			console.error("Search failed:", error);
		} finally {
			setLoading(false);
		}
	};

	return (
		<Paper elevation = {2}
	sx = {
	{
		p: 3
	}
}>
	<Grid container
	spacing = {3} >
		<Grid item
	xs = {12} >
		<TextField
			fullWidth
	label = "Search Query"
	value = {query}
	onChange = {(e)
=>
	setQuery(e.target.value)
}
	onKeyPress = {(e)
=>
	e.key === "Enter" && handleSearch()
}
	/>
	< /Grid>

	< Grid
	item
	xs = {12} >
	<Button
		variant = "contained"
	onClick = {handleSearch}
	disabled = {loading}
	startIcon = {
		loading ? <CircularProgress size = {20} /> : null}
			>
			{loading ? "Searching..." : "Search"}
			< /Button>
			< /Grid>

	{
		results && (
			<Grid item
		xs = {12} >
		<Typography variant = "h6"
		gutterBottom >
		Found
		{
			results.total
		}
		results in {results.took}
		ms
		< /Typography>

		{
			results.hits.map((hit, index) => (
				<Paper key = {index}
			elevation = {1}
			sx = {
			{
				p: 2, mb
			:
				2
			}
		}>
			<Typography variant = "h6" > {hit.document.title} < /Typography>
				< Typography
			variant = "body2"
			color = "text.secondary" >
				Score
		:
			{
				hit.score.toFixed(2)
			}
			</Typography>
			{
				Object.entries(hit.field_matches).map(([field, matches]) => (
					<div key = {field} >
					<Typography variant = "caption" > {field}
			:
				</Typography>
				{
					matches.map((match, idx) => (
						<Chip
							key = {idx}
					label = {match}
					size = "small"
					sx = {
					{
						ml: 1
					}
				}
					/>
				))
				}
				</div>
			))
			}
			</Paper>
		))
		}
		</Grid>
	)
	}
	</Grid>
	< /Paper>
)
	;
}
	;
```

## 🚀 Implementation Steps

### Phase 1: Foundation (Week 1)

1. Set up project with chosen framework
2. Configure API client and TypeScript types
3. Create basic layout and routing
4. Implement index listing page

### Phase 2: Core Features (Week 2-3)

1. Index management (CRUD operations)
2. Document upload interface
3. Basic search functionality
4. Results display

### Phase 3: Advanced Features (Week 4-5)

1. Advanced filtering interface
2. Search analytics
3. Export functionality
4. Performance monitoring

### Phase 4: Polish & Testing (Week 6)

1. Error handling and validation
2. Unit and integration tests
3. Performance optimization
4. Documentation

## 📊 Monitoring & Analytics

### Metrics to Track

- Search response times
- Popular search queries
- Index usage statistics
- Error rates
- User engagement

### Implementation

```typescript
// utils/analytics.ts
export class SearchAnalytics {
  static trackSearch(
    indexName: string,
    query: string,
    resultCount: number,
    responseTime: number
  ) {
    // Send to analytics service
    console.log(
      `Search: ${query} in ${indexName} -> ${resultCount} results (${responseTime}ms)`
    );
  }

  static trackIndexOperation(
    operation: string,
    indexName: string,
    success: boolean
  ) {
    console.log(
      `Index ${operation}: ${indexName} -> ${success ? "success" : "failed"}`
    );
  }
}
```

## 🔒 Security Considerations

### Authentication (Future Enhancement)

- JWT token management
- Role-based access control
- API key management

### Data Validation

- Input sanitization
- Schema validation
- File upload security

## 📱 Responsive Design

### Mobile-First Approach

- Touch-friendly interface
- Responsive grid layouts
- Progressive web app features

### Breakpoints

```scss
// styles/breakpoints.scss
$mobile: 768px;
$tablet: 1024px;
$desktop: 1200px;
```

This comprehensive guide provides everything you need to build a professional dashboard for your Go Search Engine. The
OpenAPI specification serves as the contract between your frontend and backend, ensuring type safety and consistency
throughout the development process.
