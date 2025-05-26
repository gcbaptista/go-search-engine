package search

import "github.com/gcbaptista/go-search-engine/model"

// candidateHit represents a document candidate during search processing
type candidateHit struct {
	doc                      model.Document
	score                    float64
	filterScore              float64
	matchedQueryTermsByField map[string]map[string]struct{} // FieldName -> queryToken -> struct{}
}
