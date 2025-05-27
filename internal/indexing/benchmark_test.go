package indexing

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/gcbaptista/go-search-engine/config"
	"github.com/gcbaptista/go-search-engine/index"
	"github.com/gcbaptista/go-search-engine/model"
	"github.com/gcbaptista/go-search-engine/store"
)

// generateTestDocuments creates a slice of test documents for benchmarking
func generateTestDocuments(count int) []model.Document {
	docs := make([]model.Document, count)
	for i := 0; i < count; i++ {
		docs[i] = model.Document{
			"documentID":  fmt.Sprintf("doc_%d", i),
			"title":       fmt.Sprintf("Test Document %d", i),
			"description": fmt.Sprintf("This is a test document number %d with some content for indexing", i),
			"tags":        []string{"test", "benchmark", fmt.Sprintf("tag_%d", i%10)},
			"category":    fmt.Sprintf("category_%d", i%5),
		}
	}
	return docs
}

// createTestService creates a new indexing service for benchmarking
func createTestService() *Service {
	settings := &config.IndexSettings{
		Name:             "benchmark_test",
		SearchableFields: []string{"title", "description", "tags"},
		FilterableFields: []string{"category"},
	}

	docStore := &store.DocumentStore{
		Docs:                   make(map[uint32]model.Document),
		ExternalIDtoInternalID: make(map[string]uint32),
		NextID:                 0,
	}

	invIndex := &index.InvertedIndex{
		Index:    make(map[string]index.PostingList),
		Settings: settings,
	}

	service, _ := NewService(invIndex, docStore)
	return service
}

// BenchmarkOriginalIndexing benchmarks the original micro-batch indexing approach
func BenchmarkOriginalIndexing(b *testing.B) {
	sizes := []int{100, 500, 1000, 5000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("docs_%d", size), func(b *testing.B) {
			docs := generateTestDocuments(size)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				service := createTestService()

				start := time.Now()
				// Force original micro-batch approach by calling addDocumentMicroBatch directly
				const microBatchSize = 10
				for j := 0; j < len(docs); j += microBatchSize {
					end := j + microBatchSize
					if end > len(docs) {
						end = len(docs)
					}
					microBatch := docs[j:end]
					if err := service.addDocumentMicroBatch(microBatch); err != nil {
						b.Fatalf("Failed to add micro-batch: %v", err)
					}
				}
				duration := time.Since(start)

				b.ReportMetric(float64(size)/duration.Seconds(), "docs/sec")
			}
		})
	}
}

// BenchmarkBulkIndexing benchmarks the new bulk indexing approach
func BenchmarkBulkIndexing(b *testing.B) {
	sizes := []int{100, 500, 1000, 5000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("docs_%d", size), func(b *testing.B) {
			docs := generateTestDocuments(size)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				service := createTestService()
				config := DefaultBulkIndexingConfig()
				config.BatchSize = 500
				config.WorkerCount = runtime.NumCPU()

				bulkIndexer := NewBulkIndexer(service, config)

				start := time.Now()
				if err := bulkIndexer.BulkAddDocuments(docs); err != nil {
					b.Fatalf("Failed to bulk add documents: %v", err)
				}
				duration := time.Since(start)

				b.ReportMetric(float64(size)/duration.Seconds(), "docs/sec")
			}
		})
	}
}

// BenchmarkAddDocumentsAutomatic benchmarks the automatic selection between micro-batch and bulk
func BenchmarkAddDocumentsAutomatic(b *testing.B) {
	sizes := []int{50, 100, 500, 1000, 5000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("docs_%d", size), func(b *testing.B) {
			docs := generateTestDocuments(size)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				service := createTestService()

				start := time.Now()
				if err := service.AddDocuments(docs); err != nil {
					b.Fatalf("Failed to add documents: %v", err)
				}
				duration := time.Since(start)

				b.ReportMetric(float64(size)/duration.Seconds(), "docs/sec")
			}
		})
	}
}

// BenchmarkReindexing benchmarks the reindexing operation
func BenchmarkReindexing(b *testing.B) {
	sizes := []int{500, 1000, 2000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("docs_%d", size), func(b *testing.B) {
			// Pre-populate the index
			service := createTestService()
			docs := generateTestDocuments(size)
			if err := service.AddDocuments(docs); err != nil {
				b.Fatalf("Failed to pre-populate index: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				config := DefaultBulkIndexingConfig()
				config.BatchSize = 1000
				config.WorkerCount = runtime.NumCPU()

				start := time.Now()
				if err := service.BulkReindex(config); err != nil {
					b.Fatalf("Failed to reindex: %v", err)
				}
				duration := time.Since(start)

				b.ReportMetric(float64(size)/duration.Seconds(), "docs/sec")
			}
		})
	}
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	sizes := []int{1000, 5000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("bulk_docs_%d", size), func(b *testing.B) {
			docs := generateTestDocuments(size)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var m1, m2 runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&m1)

				service := createTestService()
				config := DefaultBulkIndexingConfig()
				config.BatchSize = 500
				config.WorkerCount = 2

				bulkIndexer := NewBulkIndexer(service, config)
				if err := bulkIndexer.BulkAddDocuments(docs); err != nil {
					b.Fatalf("Failed to bulk add documents: %v", err)
				}

				runtime.GC()
				runtime.ReadMemStats(&m2)

				b.ReportMetric(float64(m2.Alloc-m1.Alloc)/1024/1024, "MB_allocated")
				b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/1024/1024, "MB_total")
			}
		})

		b.Run(fmt.Sprintf("original_docs_%d", size), func(b *testing.B) {
			docs := generateTestDocuments(size)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var m1, m2 runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&m1)

				service := createTestService()

				// Force original approach
				const microBatchSize = 10
				for j := 0; j < len(docs); j += microBatchSize {
					end := j + microBatchSize
					if end > len(docs) {
						end = len(docs)
					}
					microBatch := docs[j:end]
					if err := service.addDocumentMicroBatch(microBatch); err != nil {
						b.Fatalf("Failed to add micro-batch: %v", err)
					}
				}

				runtime.GC()
				runtime.ReadMemStats(&m2)

				b.ReportMetric(float64(m2.Alloc-m1.Alloc)/1024/1024, "MB_allocated")
				b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/1024/1024, "MB_total")
			}
		})
	}
}

// BenchmarkConcurrency benchmarks different worker counts
func BenchmarkConcurrency(b *testing.B) {
	docs := generateTestDocuments(2000)
	workerCounts := []int{1, 2, 4, 8, runtime.NumCPU()}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("workers_%d", workers), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				service := createTestService()
				config := DefaultBulkIndexingConfig()
				config.BatchSize = 500
				config.WorkerCount = workers

				bulkIndexer := NewBulkIndexer(service, config)

				start := time.Now()
				if err := bulkIndexer.BulkAddDocuments(docs); err != nil {
					b.Fatalf("Failed to bulk add documents: %v", err)
				}
				duration := time.Since(start)

				b.ReportMetric(float64(len(docs))/duration.Seconds(), "docs/sec")
			}
		})
	}
}

// BenchmarkBatchSizes benchmarks different batch sizes
func BenchmarkBatchSizes(b *testing.B) {
	docs := generateTestDocuments(2000)
	batchSizes := []int{100, 500, 1000, 2000}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", batchSize), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				service := createTestService()
				config := DefaultBulkIndexingConfig()
				config.BatchSize = batchSize
				config.WorkerCount = runtime.NumCPU()

				bulkIndexer := NewBulkIndexer(service, config)

				start := time.Now()
				if err := bulkIndexer.BulkAddDocuments(docs); err != nil {
					b.Fatalf("Failed to bulk add documents: %v", err)
				}
				duration := time.Since(start)

				b.ReportMetric(float64(len(docs))/duration.Seconds(), "docs/sec")
			}
		})
	}
}
