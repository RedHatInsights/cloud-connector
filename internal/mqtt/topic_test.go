package mqtt

import (
	"fmt"
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

func init() {
	logger.InitLogger()
}

func buildTopicToVerify(prefix string, clientId domain.ClientID) string {
	return prefix + "/insights/" + string(clientId) + "/control/out"
}

func TestTopicVerifier(t *testing.T) {
	var expectedClientId domain.ClientID = "98765"

	testCases := []struct {
		prefix        string
		prefixInTopic string
	}{
		{"staging", "staging"},
		{"", "redhat"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("prefix = '%s'", tc.prefix), func(t *testing.T) {
			topicVerifier := NewTopicVerifier(tc.prefix)
			topicToVerify := buildTopicToVerify(tc.prefixInTopic, expectedClientId)
			_, actualClientId, err := topicVerifier.VerifyIncomingTopic(topicToVerify)

			if err != nil {
				t.Fatal("unexpected error ", err)
			}

			if actualClientId != expectedClientId {
				t.Fatalf("expected client id %s, but got %s!", expectedClientId, actualClientId)
			}
		})
	}
}

func TestTopicVerifierPrefixMismatch(t *testing.T) {
	var expectedClientId domain.ClientID = "98765"

	testCases := []struct {
		verifierPrefix string
		topicPrefix    string
	}{
		{"", "NOT/"},
		{"staging", ""},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("verifierPrefix = '%s'", tc.verifierPrefix), func(t *testing.T) {
			topicVerifier := NewTopicVerifier(tc.verifierPrefix)
			topicToVerify := buildTopicToVerify(tc.topicPrefix, expectedClientId)
			_, _, err := topicVerifier.VerifyIncomingTopic(topicToVerify)

			if err == nil {
				t.Fatal("expected an error, did not receive an error")
			}
		})
	}
}

func TestTopicBuilderOutgoingTopic(t *testing.T) {
	var expectedClientId domain.ClientID = "98765"
	var expectedPrefix string = "staging"

	testCases := []struct {
		prefix         string
		expectedTopic  string
		buildTopicFunc func(clientId domain.ClientID, tb *TopicBuilder) string
	}{
		{"", "redhat/insights/" + string(expectedClientId) + "/data/in", buildOutgoingDataTopic},
		{expectedPrefix, expectedPrefix + "/insights/" + string(expectedClientId) + "/data/in", buildOutgoingDataTopic},

		{"", "redhat/insights/" + string(expectedClientId) + "/control/in", buildOutgoingControlTopic},
		{expectedPrefix, expectedPrefix + "/insights/" + string(expectedClientId) + "/control/in", buildOutgoingControlTopic},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("subtest '%d'", i), func(t *testing.T) {
			topicBuilder := NewTopicBuilder(tc.prefix)
			actualTopic := tc.buildTopicFunc(expectedClientId, topicBuilder)

			if actualTopic != tc.expectedTopic {
				t.Fatalf("expected topic %s, but got %s!", tc.expectedTopic, actualTopic)
			}
		})
	}
}

func buildOutgoingDataTopic(clientId domain.ClientID, tb *TopicBuilder) string {
	return tb.BuildOutgoingDataTopic(clientId)
}

func buildOutgoingControlTopic(clientId domain.ClientID, tb *TopicBuilder) string {
	return tb.BuildOutgoingControlTopic(clientId)
}

func TestTopicBuilderIncomingWildcardTopic(t *testing.T) {
	var expectedPrefix string = "staging"

	testCases := []struct {
		prefix         string
		expectedTopic  string
		buildTopicFunc func(tb *TopicBuilder) string
	}{
		{"", "redhat/insights/+/data/out", buildIncomingWildcardDataTopic},
		{expectedPrefix, expectedPrefix + "/insights/+/data/out", buildIncomingWildcardDataTopic},

		{"", "redhat/insights/+/control/out", buildIncomingWildcardControlTopic},
		{expectedPrefix, expectedPrefix + "/insights/+/control/out", buildIncomingWildcardControlTopic},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("subtest %d", i), func(t *testing.T) {
			topicBuilder := NewTopicBuilder(tc.prefix)
			actualTopic := tc.buildTopicFunc(topicBuilder)

			if actualTopic != tc.expectedTopic {
				t.Fatalf("expected topic %s, but got %s!", tc.expectedTopic, actualTopic)
			}
		})
	}
}

func buildIncomingWildcardDataTopic(tb *TopicBuilder) string {
	return tb.BuildIncomingWildcardDataTopic()
}

func buildIncomingWildcardControlTopic(tb *TopicBuilder) string {
	return tb.BuildIncomingWildcardControlTopic()
}
