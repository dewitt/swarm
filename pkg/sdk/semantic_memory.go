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
	geminiDir := filepath.Join(workspaceDir, ".gemini")
	_ = os.MkdirAll(geminiDir, 0o755)
	dbPath := filepath.Join(geminiDir, "state.db")

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
		// Only create FTS5 tables and triggers if we verified it is fully supported by the driver
		db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS semantic_facts_fts USING fts5(fact, content='semantic_facts', content_rowid='id');`)

		db.Exec(`
			CREATE TRIGGER IF NOT EXISTS semantic_facts_ai AFTER INSERT ON semantic_facts BEGIN
				INSERT INTO semantic_facts_fts(rowid, fact) VALUES (new.id, new.fact);
			END;
		`)
		db.Exec(`
			CREATE TRIGGER IF NOT EXISTS semantic_facts_ad AFTER DELETE ON semantic_facts BEGIN
				INSERT INTO semantic_facts_fts(semantic_facts_fts, rowid, fact) VALUES ('delete', old.id, old.fact);
			END;
		`)
		db.Exec(`
			CREATE TRIGGER IF NOT EXISTS semantic_facts_au AFTER UPDATE ON semantic_facts BEGIN
				INSERT INTO semantic_facts_fts(semantic_facts_fts, rowid, fact) VALUES ('delete', old.id, old.fact);
				INSERT INTO semantic_facts_fts(rowid, fact) VALUES (new.id, new.fact);
			END;
		`)

		// Rebuild the FTS index to ensure it is fully synchronized with the underlying semantic_facts table.
		// This guarantees that if FTS was previously disabled and is now enabled, existing facts are indexed.
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

		// 2. Convert spaces to OR operators so that if ANY keyword matches, the fact is retrieved.
		// This makes retrieval much more robust for natural language prompts.
		parts := strings.Fields(safeQuery)
		if len(parts) > 0 {
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
		for _, word := range strings.Fields(safeQuery) {
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

