package store

import (
	"fmt"

	bolt "go.etcd.io/bbolt"
)

const currentSchemaVersion = "1"

// RunMigrations checks the schema version and applies any necessary migrations.
func (s *BoltStore) RunMigrations() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketMeta)
		v := b.Get([]byte("schema_version"))

		if v == nil {
			// Fresh database, set initial version
			return b.Put([]byte("schema_version"), []byte(currentSchemaVersion))
		}

		version := string(v)
		if version == currentSchemaVersion {
			return nil
		}

		// Future migrations would go here
		// switch version {
		// case "1":
		//     migrate_1_to_2(tx)
		//     fallthrough
		// case "2":
		//     ...
		// }

		return fmt.Errorf("unknown schema version %q, expected %q", version, currentSchemaVersion)
	})
}
