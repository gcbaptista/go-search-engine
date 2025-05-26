package model

// Document is a flexible map representing a JSON document.
// The documentID is the only required field for document identification.
// Other fields like "title", "popularity", etc., are accessed by their string keys and depend on index configuration.
// Example: doc["title"], doc["popularity"]
type Document map[string]interface{}

// GetDocumentID returns the documentID if it's stored in the document map under "documentID" key.
func (d Document) GetDocumentID() (string, bool) {
	if id, ok := d["documentID"]; ok {
		if str, sok := id.(string); sok {
			if str != "" {
				return str, true
			}
		}
	}
	return "", false
}

// GetPopularity returns the Popularity if it's stored and is a float64.
func (d Document) GetPopularity() (float64, bool) {
	if pop, ok := d["Popularity"]; ok { // Note: Case-sensitive key matching
		if p, pok := pop.(float64); pok {
			return p, true
		}
	}
	return 0, false
}
