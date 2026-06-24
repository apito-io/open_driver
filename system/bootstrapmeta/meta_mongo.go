//go:build !cloudflare

package bootstrapmeta

import (
	"errors"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

func isMongoUserLookupMiss(err error) bool {
	return errors.Is(err, mongo.ErrNoDocuments)
}
