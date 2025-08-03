package database

import (
	"log"
	"subtrackr/internal/models"

	"gorm.io/gorm"
)

// RunMigrations executes all database migrations
func RunMigrations(db *gorm.DB) error {
	// Auto-migrate the schema
	err := db.AutoMigrate(&models.Category{}, &models.Subscription{}, &models.Settings{}, &models.APIKey{})
	if err != nil {
		return err
	}

	// Run specific migrations
	migrations := []func(*gorm.DB) error{
		migrateCategoriesToDynamic,
	}

	for _, migration := range migrations {
		if err := migration(db); err != nil {
			return err
		}
	}

	return nil
}

// migrateCategoriesToDynamic handles the v0.3.0 migration from string categories to category IDs
func migrateCategoriesToDynamic(db *gorm.DB) error {
	// Check if migration is needed by looking for a temporary column
	var count int64
	db.Raw("SELECT COUNT(*) FROM pragma_table_info('subscriptions') WHERE name='category'").Scan(&count)
	
	if count == 0 {
		// Migration already completed
		return nil
	}

	log.Println("Running migration: Converting categories to dynamic system...")

	// First ensure default categories exist
	defaultCategories := []string{"Entertainment", "Productivity", "Storage", "Software", "Fitness", "Education", "Food", "Travel", "Business", "Other"}
	var categories []models.Category
	db.Find(&categories)
	
	if len(categories) == 0 {
		for _, name := range defaultCategories {
			db.Create(&models.Category{Name: name})
		}
		db.Find(&categories) // Reload categories
	}

	// Create category map
	categoryMap := make(map[string]uint)
	for _, cat := range categories {
		categoryMap[cat.Name] = cat.ID
	}

	// Get all subscriptions that need migration
	type OldSubscription struct {
		ID       uint
		Category string
	}
	
	var oldSubs []OldSubscription
	db.Table("subscriptions").Select("id, category").Scan(&oldSubs)

	// Update each subscription with the appropriate category_id
	for _, sub := range oldSubs {
		if sub.Category != "" {
			if catID, exists := categoryMap[sub.Category]; exists {
				db.Table("subscriptions").Where("id = ?", sub.ID).Update("category_id", catID)
			} else {
				// If category doesn't exist, use "Other"
				if otherID, exists := categoryMap["Other"]; exists {
					db.Table("subscriptions").Where("id = ?", sub.ID).Update("category_id", otherID)
				}
			}
		}
	}

	// Note: We can't drop the old 'category' column in SQLite without recreating the table
	// This would be handled differently in production with proper migration tools
	
	log.Println("Migration completed: Categories converted to dynamic system")
	return nil
}