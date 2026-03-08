package sdk

import (
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type SemanticFact struct {
	ID        uint   `gorm:"primarykey"`
	Fact      string `gorm:"type:text"`
	CreatedAt int64  `gorm:"autoCreateTime"`
}

type SemanticMemory struct {
	db         *gorm.DB
	ftsEnabled bool
}

func NewSemanticMemory(workspaceDir string) (*SemanticMemory, error) {
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
	// Create FTS5 virtual table if not exists
	err = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS semantic_facts_fts USING fts5(fact, content='semantic_facts', content_rowid='id');`).Error
	if err != nil {
		ftsEnabled = false
	} else {
		// Only create triggers if the FTS table successfully created
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
	}

	return &SemanticMemory{db: db, ftsEnabled: ftsEnabled}, nil
}

func (sm *SemanticMemory) Commit(fact string) error {
	return sm.db.Create(&SemanticFact{Fact: fact}).Error
}

func (sm *SemanticMemory) Retrieve(query string, limit int) ([]string, error) {
	var facts []SemanticFact
	var err error

	if sm.ftsEnabled {
		err = sm.db.Raw(`
			SELECT sf.* FROM semantic_facts sf
			JOIN semantic_facts_fts fts ON sf.id = fts.rowid
			WHERE semantic_facts_fts MATCH ?
			ORDER BY rank
			LIMIT ?
		`, query, limit).Scan(&facts).Error
	} else {
		// Fallback to simple LIKE search if FTS5 is not available in the binary
		err = sm.db.Where("fact LIKE ?", "%"+query+"%").Order("created_at desc").Limit(limit).Find(&facts).Error
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

func (sm *SemanticMemory) List(limit int) ([]string, error) {
	var facts []SemanticFact
	err := sm.db.Order("created_at desc").Limit(limit).Find(&facts).Error
	if err != nil {
		return nil, err
	}

	var results []string
	for _, f := range facts {
		results = append(results, f.Fact)
	}
	return results, nil
}
