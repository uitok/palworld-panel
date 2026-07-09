package indexer

import "time"

func UTCNow() string {
	return time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
}
