package connection_repository

import (
	"database/sql"
	"encoding/json"

	"github.com/RedHatInsights/cloud-connector/internal/domain"

	"github.com/sirupsen/logrus"
)

func deserializeDispatchers(log *logrus.Entry, serializedDispatchers sql.NullString) domain.Dispatchers {
	var dispatchers domain.Dispatchers
	if serializedDispatchers.Valid {
		err := json.Unmarshal([]byte(serializedDispatchers.String), &dispatchers)
		if err != nil {
			log.WithFields(logrus.Fields{"error": err}).Error("Unable to unmarshal dispatchers from database")
		}
	}
	return dispatchers
}

func deserializeCanonicalFacts(log *logrus.Entry, serializedCanonicalFacts sql.NullString) domain.CanonicalFacts {
	var canonicalFacts domain.CanonicalFacts
	if serializedCanonicalFacts.Valid {
		err := json.Unmarshal([]byte(serializedCanonicalFacts.String), &canonicalFacts)
		if err != nil {
			log.WithFields(logrus.Fields{"error": err}).Error("Unable to unmarshal canonical facts from database")
		}
	}
	return canonicalFacts
}

func deserializeTags(log *logrus.Entry, serializedTags sql.NullString) domain.Tags {
	var tags domain.Tags
	if serializedTags.Valid {
		err := json.Unmarshal([]byte(serializedTags.String), &tags)
		if err != nil {
			log.WithFields(logrus.Fields{"error": err}).Error("Unable to unmarshal tags from database")
		}
	}
	return tags
}
