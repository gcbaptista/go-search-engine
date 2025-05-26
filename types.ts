/**
 * TypeScript type definitions for Go Search Engine API
 * Generated from OpenAPI 3.0.3 specification
 */

// ===== Core Types =====

export interface RankingCriterion {
	field: string;
	order: 'asc' | 'desc';
}

export interface IndexSettings {
	name: string;
	searchable_fields: string[]; // Fields in priority order - search engine exhausts each field before proceeding to next
	filterable_fields: string[];
	ranking_criteria: RankingCriterion[];
	min_word_size_for_1_typo: number;
	min_word_size_for_2_typos: number;
	fields_without_prefix_search: string[];
	no_typo_tolerance_fields: string[]; // Fields for which typo tolerance is disabled
	distinct_field?: string; // Field to use for deduplication
}

export interface IndexSettingsUpdate {
	fields_without_prefix_search?: string[];
	no_typo_tolerance_fields?: string[];
	distinct_field?: string;
	searchable_fields?: string[];
	filterable_fields?: string[];
	ranking_criteria?: RankingCriterion[];
	min_word_size_for_1_typo?: number;
	min_word_size_for_2_typos?: number;
}

export interface RenameIndexRequest {
	new_name: string;
}

export interface RenameIndexResponse {
	message: string;
	old_name: string;
	new_name: string;
}

export interface Document {
	documentID: string; // Required field for document identification - can be any non-empty string
	// Schema-agnostic: documentID is the only required field
	// All other fields depend on index configuration (searchable_fields, filterable_fields)
	[key: string]: any; // Allow any additional properties
}

export interface SearchRequest {
	query: string;
	filters?: Record<string, any>;
	page?: number;
	page_size?: number;
}

export interface HitInfo {
	num_typos: number;
	number_exact_words: number;
}

export interface SearchHit {
	document: Document;
	score: number;
	field_matches: Record<string, string[]>;
	hit_info: HitInfo;
}

export interface SearchResult {
	hits: SearchHit[];
	total: number;
	page: number;
	page_size: number;
	took: number; // milliseconds
	query_id: string; // unique UUID for this search query
}

// ===== API Response Types =====

export interface SuccessMessage {
	message: string;
}

export interface ErrorResponse {
	error: string;
}

export interface IndexListResponse {
	indexes: string[];
	count: number;
}

export interface UpdateIndexSettingsResponse {
	message: string;
	warning?: string;
	reindexed?: boolean; // Whether documents were automatically reindexed
}

// ===== Filter Types =====

export type FilterOperator =
	| '_exact'
	| '_ne'
	| '_gt'
	| '_gte'
	| '_lt'
	| '_lte'
	| '_contains'
	| '_ncontains'
	| '_contains_any_of';

export interface Filter {
	field: string;
	operator: FilterOperator | '';
	value: any;
}

// ===== Extended Types for Dashboard =====

export interface IndexWithStats extends IndexSettings {
	document_count?: number;
	size_bytes?: number;
	created_at?: string;
	last_updated?: string;
}

export interface SearchHistory {
	id: string;
	timestamp: string;
	index_name: string;
	query: string;
	filters: Record<string, any>;
	result_count: number;
	response_time: number;
}

export interface IndexStatistics {
	name: string;
	document_count: number;
	size_bytes: number;
	total_searches: number;
	avg_response_time: number;
	popular_queries: { query: string; count: number }[];
}

// ===== UI State Types =====

export interface PaginationState {
	page: number;
	pageSize: number;
	total: number;
}

export interface LoadingState {
	isLoading: boolean;
	error?: string | null;
}

export interface SearchFormState {
	query: string;
	filters: Filter[];
	page: number;
	pageSize: number;
}

// ===== Component Props Types =====

export interface IndexListProps {
	indexes: IndexWithStats[];
	onEdit: (index: IndexSettings) => void;
	onDelete: (indexName: string) => void;
	onView: (indexName: string) => void;
	loading?: boolean;
}

export interface IndexFormProps {
	index?: IndexSettings;
	onSubmit: (index: IndexSettings) => Promise<void>;
	onCancel: () => void;
	loading?: boolean;
}

export interface DocumentUploadProps {
	indexName: string;
	onUpload: (documents: Document | Document[]) => Promise<void>;
	loading?: boolean;
}

export interface SearchInterfaceProps {
	indexName: string;
	availableFields: {
		searchable: string[];
		filterable: string[];
	};
	onSearch?: (results: SearchResult) => void;
}

export interface SearchResultsProps {
	results: SearchResult;
	onDocumentClick?: (document: Document) => void;
	onExport?: () => void;
	loading?: boolean;
}

export interface FilterPanelProps {
	availableFields: string[];
	filters: Filter[];
	onFiltersChange: (filters: Filter[]) => void;
}

// ===== API Client Types =====

export interface ApiClientConfig {
	baseURL: string;
	timeout?: number;
	headers?: Record<string, string>;
}

export interface ApiError {
	status: number;
	message: string;
	details?: any;
}

// ===== Dashboard State Types =====

export interface DashboardState {
	indexes: {
		list: IndexWithStats[];
		current: string | null;
		loading: boolean;
		error: string | null;
	};
	search: {
		results: SearchResult | null;
		history: SearchHistory[];
		loading: boolean;
		error: string | null;
	};
	documents: {
		uploading: boolean;
		error: string | null;
	};
	ui: {
		sidebarOpen: boolean;
		theme: 'light' | 'dark';
	};
}

// ===== Form Validation Types =====

export interface ValidationRule {
	required?: boolean;
	minLength?: number;
	maxLength?: number;
	pattern?: RegExp;
	custom?: (value: any) => string | null;
}

export interface FormField {
	name: string;
	label: string;
	type: 'text' | 'number' | 'array' | 'select' | 'checkbox';
	validation?: ValidationRule;
	options?: { label: string; value: any }[];
}

export interface FormErrors {
	[fieldName: string]: string[];
}

// ===== Export/Import Types =====

export interface ExportOptions {
	format: 'json' | 'csv' | 'excel';
	fields?: string[];
	filename?: string;
}

export interface ImportOptions {
	format: 'json' | 'csv';
	fieldMapping?: Record<string, string>;
	hasHeader?: boolean;
}

// ===== Real-time Types =====

export interface WebSocketMessage {
	type: 'search_update' | 'index_update' | 'document_update' | 'error';
	payload: any;
	timestamp: string;
}

export interface NotificationMessage {
	id: string;
	type: 'success' | 'error' | 'warning' | 'info';
	title: string;
	message: string;
	timestamp: string;
	autoHide?: boolean;
}

// ===== Type Guards =====

export function isSearchResult(obj: any): obj is SearchResult {
	return obj &&
		Array.isArray(obj.hits) &&
		typeof obj.total === 'number' &&
		typeof obj.page === 'number' &&
		typeof obj.page_size === 'number' &&
		typeof obj.took === 'number' &&
		typeof obj.query_id === 'string';
}

export function isErrorResponse(obj: any): obj is ErrorResponse {
	return obj && typeof obj.error === 'string';
}

export function isIndexSettings(obj: any): obj is IndexSettings {
	return obj &&
		typeof obj.name === 'string' &&
		Array.isArray(obj.searchable_fields) &&
		Array.isArray(obj.filterable_fields);
}

// ===== Utility Types =====

export type SortDirection = 'asc' | 'desc';

export type ViewMode = 'table' | 'cards' | 'list';

export type ThemeMode = 'light' | 'dark' | 'auto';

// ===== Constants =====

export const FILTER_OPERATORS: { value: FilterOperator | ''; label: string }[] = [
	{value: '', label: 'Equals'},
	{value: '_ne', label: 'Not equals'},
	{value: '_gt', label: 'Greater than'},
	{value: '_gte', label: 'Greater than or equal'},
	{value: '_lt', label: 'Less than'},
	{value: '_lte', label: 'Less than or equal'},
	{value: '_contains', label: 'Contains'},
	{value: '_ncontains', label: 'Does not contain'},
	{value: '_contains_any_of', label: 'Contains any of'},
];

export const DEFAULT_PAGE_SIZE = 10;
export const MAX_PAGE_SIZE = 100;
export const DEFAULT_PAGINATION: PaginationState = {
	page: 1,
	pageSize: DEFAULT_PAGE_SIZE,
	total: 0,
};

// ===== Example Data Types =====
// Note: These examples show possible document structures, but all fields are optional
// Documents are completely schema-agnostic - any field structure is allowed

export const EXAMPLE_MOVIE_INDEX: IndexSettings = {
	name: 'movies',
	searchable_fields: ['title', 'cast', 'plot', 'genres'],
	filterable_fields: ['year', 'rating', 'director', 'genres'],
	ranking_criteria: [
		{field: 'popularity', order: 'desc'},
		{field: 'rating', order: 'desc'},
	],
	min_word_size_for_1_typo: 4,
	min_word_size_for_2_typos: 7,
	fields_without_prefix_search: [],
	no_typo_tolerance_fields: [],
};

export const EXAMPLE_MOVIE_DOCUMENT: Document = {
	documentID: 'movie_lotr_fellowship_2001',
	title: 'The Lord of the Rings: The Fellowship of the Ring',
	cast: ['Elijah Wood', 'Ian McKellen', 'Viggo Mortensen'],
	genres: ['Fantasy', 'Adventure'],
	year: 2001,
	rating: 8.8,
	director: 'Peter Jackson',
	plot: 'A meek Hobbit from the Shire and eight companions set out on a journey to destroy the powerful One Ring and save Middle-earth from the Dark Lord Sauron.',
	popularity: 95.5,
};

// Example of different document structures (schema-agnostic)
export const EXAMPLE_PRODUCT_DOCUMENT: Document = {
	documentID: 'product_headphones_wireless_tech_001',
	name: 'Wireless Headphones',
	brand: 'TechBrand',
	price: 199.99,
	category: ['Electronics', 'Audio'],
	specifications: {
		battery_life: '30 hours',
		connectivity: 'Bluetooth 5.0'
	},
	in_stock: true
};

export const EXAMPLE_ARTICLE_DOCUMENT: Document = {
	documentID: 'article_breaking_news_tech_2024_01_15',
	headline: 'Breaking News Story',
	content: 'Article content here...',
	author: 'Jane Doe',
	published_date: '2024-01-15',
	tags: ['news', 'technology'],
	word_count: 1250
};

export const EXAMPLE_SEARCH_REQUEST: SearchRequest = {
	query: 'lord rings',
	filters: {
		year_gte: 2000,
		rating_gte: 8.0,
		genres_contains: 'Fantasy',
	},
	page: 1,
	page_size: 10,
};

// ===== New API Response Types =====

export interface HealthCheckResponse {
	status: string;
	service: string;
	timestamp: string;
}

export interface IndexStatsResponse {
	name: string;
	document_count: number;
	searchable_fields: string[];
	filterable_fields: string[];
	typo_settings: {
		min_word_size_for_1_typo: number;
		min_word_size_for_2_typos: number;
	};
	field_settings: {
		fields_without_prefix_search: string[];
		no_typo_tolerance_fields: string[];
		distinct_field: string;
	};
}

export interface DocumentListResponse {
	documents: Document[];
	total: number;
	page: number;
	page_size: number;
	pages: number;
}

export interface DocumentListRequest {
	page?: number;
	page_size?: number;
} 