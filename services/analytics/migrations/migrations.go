package migrations

import (
	"tachyon-messenger/shared/database"
)

// RunMigrations runs custom SQL migrations for analytics service
func RunMigrations(db *database.DB) error {
	migrations := []string{
		// Create indexes for analytics_events table (if not handled by GORM)
		`CREATE INDEX IF NOT EXISTS idx_events_timestamp ON analytics_events(timestamp DESC);`,

		// Create composite indexes for better query performance
		`CREATE INDEX IF NOT EXISTS idx_events_type_user ON analytics_events(event_type, user_id, timestamp DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_events_category_dept ON analytics_events(event_category, department_id, timestamp DESC);`,

		// Create indexes for aggregated_metrics
		`CREATE INDEX IF NOT EXISTS idx_metrics_period ON aggregated_metrics(period_type, period_start DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_name_dept ON aggregated_metrics(metric_name, department_id, period_start DESC);`,

		// Create function to update updated_at timestamp
		`CREATE OR REPLACE FUNCTION update_analytics_updated_at()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;`,

		// Create trigger for aggregated_metrics
		`DROP TRIGGER IF EXISTS trigger_update_aggregated_metrics_updated_at ON aggregated_metrics;`,
		`CREATE TRIGGER trigger_update_aggregated_metrics_updated_at
		BEFORE UPDATE ON aggregated_metrics
		FOR EACH ROW
		EXECUTE FUNCTION update_analytics_updated_at();`,

		// Create trigger for user_activity
		`DROP TRIGGER IF EXISTS trigger_update_user_activity_updated_at ON user_activity;`,
		`CREATE TRIGGER trigger_update_user_activity_updated_at
		BEFORE UPDATE ON user_activity
		FOR EACH ROW
		EXECUTE FUNCTION update_analytics_updated_at();`,

		// Create trigger for department_stats
		`DROP TRIGGER IF EXISTS trigger_update_department_stats_updated_at ON department_stats;`,
		`CREATE TRIGGER trigger_update_department_stats_updated_at
		BEFORE UPDATE ON department_stats
		FOR EACH ROW
		EXECUTE FUNCTION update_analytics_updated_at();`,

		// Create partitioning for analytics_events by month (optional, for large datasets)
		// This can be uncommented when the table grows large
		/*
		`CREATE TABLE IF NOT EXISTS analytics_events_2024_01 PARTITION OF analytics_events
		FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');`,
		*/
	}

	for _, migration := range migrations {
		if err := db.DB.Exec(migration).Error; err != nil {
			return err
		}
	}

	return nil
}
