package sdk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var stopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true, "be": true, "but": true, "by": true,
	"for": true, "if": true, "in": true, "into": true, "is": true, "it": true,
	"no": true, "not": true, "of": true, "on": true, "or": true, "such": true,
	"that": true, "the": true, "their": true, "then": true, "there": true, "these": true,
	"they": true, "this": true, "to": true, "was": true, "will": true, "with": true,
	"what": true, "where": true, "who": true, "why": true, "how": true, "when": true,
	"does": true, "do": true, "did": true, "can": true, "could": true, "should": true, "would": true,
}

type SemanticFact struct {
	ID        uint   `gorm:"primarykey"`
	Fact      string `gorm:"type:text"`
	CreatedAt int64  `gorm:"autoCreateTime"`
}

type sqliteSemanticMemory struct {
	db         *gorm.DB
	ftsEnabled bool
}

func NewSemanticMemory(workspaceDir string) (SemanticMemory, error) {
	swarmDir := filepath.Join(workspaceDir, ".swarm")
	_ = os.MkdirAll(swarmDir, 0o755)
	dbPath := filepath.Join(swarmDir, "state.db")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open semantic memory: %w", err)
	}

	// Create regular table
	err = db.AutoMigrate(&SemanticFact{})
	if err != nil {
		return nil, err
	}

	ftsEnabled := true
	// Test if FTS5 is actually available in the sqlite driver before doing ANYTHING with it
	err = db.Exec("CREATE VIRTUAL TABLE test_fts_support USING fts5(content);").Error
	if err != nil {
		ftsEnabled = false
	} else {
		db.Exec("DROP TABLE test_fts_support;")
	}

	if ftsEnabled {
		// Drop existing FTS table and triggers to apply schema changes (like tokenize='porter')
		db.Exec(`DROP TRIGGER IF EXISTS semantic_facts_ai;`)
		db.Exec(`DROP TRIGGER IF EXISTS semantic_facts_ad;`)
		db.Exec(`DROP TRIGGER IF EXISTS semantic_facts_au;`)
		db.Exec(`DROP TABLE IF EXISTS semantic_facts_fts;`)

		// Recreate with porter stemmer
		db.Exec(`CREATE VIRTUAL TABLE semantic_facts_fts USING fts5(fact, content='semantic_facts', content_rowid='id', tokenize='porter');`)

		db.Exec(`
			CREATE TRIGGER semantic_facts_ai AFTER INSERT ON semantic_facts BEGIN
				INSERT INTO semantic_facts_fts(rowid, fact) VALUES (new.id, new.fact);
			END;
		`)
		db.Exec(`
			CREATE TRIGGER semantic_facts_ad AFTER DELETE ON semantic_facts BEGIN
				INSERT INTO semantic_facts_fts(semantic_facts_fts, rowid, fact) VALUES ('delete', old.id, old.fact);
			END;
		`)
		db.Exec(`
			CREATE TRIGGER semantic_facts_au AFTER UPDATE ON semantic_facts BEGIN
				INSERT INTO semantic_facts_fts(semantic_facts_fts, rowid, fact) VALUES ('delete', old.id, old.fact);
				INSERT INTO semantic_facts_fts(rowid, fact) VALUES (new.id, new.fact);
			END;
		`)

		// Rebuild the FTS index to ensure it is fully synchronized with the underlying semantic_facts table.
		db.Exec(`INSERT INTO semantic_facts_fts(semantic_facts_fts) VALUES('rebuild');`)
	}

	return &sqliteSemanticMemory{db: db, ftsEnabled: ftsEnabled}, nil
}

func (sm *sqliteSemanticMemory) Commit(fact string) error {
	return sm.db.Create(&SemanticFact{Fact: fact}).Error
}

func (sm *sqliteSemanticMemory) Retrieve(query string, limit int) ([]string, error) {
	var facts []SemanticFact
	var err error

	if sm.ftsEnabled {
		// Aggressive sanitization for FTS5 natural language:
		// Remove all characters except letters, numbers, and spaces
		var sb strings.Builder
		for _, r := range query {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' {
				sb.WriteRune(r)
			} else {
				sb.WriteRune(' ') // Replace special chars with space
			}
		}
		safeQuery := sb.String()

		// 2. Filter out common stopwords and convert spaces to OR operators
		// This makes retrieval much more robust for natural language prompts.
		parts := strings.Fields(safeQuery)
		var filteredParts []string
		for _, p := range parts {
			if !stopWords[strings.ToLower(p)] {
				filteredParts = append(filteredParts, p)
			}
		}

		if len(filteredParts) > 0 {
			safeQuery = strings.Join(filteredParts, " OR ")
		} else {
			// Fallback: if everything was a stopword, just use the original parts
			safeQuery = strings.Join(parts, " OR ")
		}

		err = sm.db.Raw(`			SELECT sf.* FROM semantic_facts sf
			JOIN semantic_facts_fts fts ON sf.id = fts.rowid
			WHERE semantic_facts_fts MATCH ?
			ORDER BY rank
			LIMIT ?
		`, safeQuery, limit).Scan(&facts).Error
	} else {
		// Fallback to simple LIKE search if FTS5 is not available in the binary
		var sb strings.Builder
		for _, r := range query {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' {
				sb.WriteRune(r)
			} else {
				sb.WriteRune(' ') // Replace special chars with space
			}
		}
		safeQuery := sb.String()

		var orQueries []string
		var orArgs []interface{}

		parts := strings.Fields(safeQuery)
		var filteredParts []string
		for _, p := range parts {
			if !stopWords[strings.ToLower(p)] {
				filteredParts = append(filteredParts, p)
			}
		}
		if len(filteredParts) == 0 {
			filteredParts = parts
		}

		for _, word := range filteredParts {
			orQueries = append(orQueries, "fact LIKE ?")
			orArgs = append(orArgs, "%"+word+"%")
		}
		if len(orQueries) > 0 {
			err = sm.db.Where(strings.Join(orQueries, " OR "), orArgs...).Order("id desc").Limit(limit).Find(&facts).Error
		} else {
			err = sm.db.Order("id desc").Limit(limit).Find(&facts).Error
		}
	}

	if err != nil {
		return nil, err
	}

	var results []string
	for _, f := range facts {
		results = append(results, f.Fact)
	}
	return results, nil
}

func (sm *sqliteSemanticMemory) List(limit int) ([]string, error) {
	var facts []SemanticFact
	err := sm.db.Order("id desc").Limit(limit).Find(&facts).Error
	if err != nil {
		return nil, err
	}

	var results []string
	for _, f := range facts {
		results = append(results, f.Fact)
	}
	return results, nil
}

// Forget removes any facts from semantic memory that contain the given keyword/substring.
// Returns the number of facts deleted.
func (sm *sqliteSemanticMemory) Forget(query string) (int, error) {
	if query == "" {
		return 0, fmt.Errorf("cannot forget empty query")
	}

	// Perform a simple LIKE deletion
	res := sm.db.Where("fact LIKE ?", "%"+query+"%").Delete(&SemanticFact{})
	if res.Error != nil {
		return 0, res.Error
	}

	return int(res.RowsAffected), nil
}

func (sm *sqliteSemanticMemory) SemanticStats() MemoryStats {
	var count int64
	sm.db.Model(&SemanticFact{}).Count(&count)

	var totalChars int64
	sm.db.Model(&SemanticFact{}).Select("sum(length(fact))").Scan(&totalChars)

	return MemoryStats{
		Name:          "Semantic Memory (Tier 3)",
		Count:         int(count),
		TokenEstimate: int(totalChars) / 4,
	}
}

func (sm *sqliteSemanticMemory) FTSEnabled() bool {
	return sm.ftsEnabled
}
