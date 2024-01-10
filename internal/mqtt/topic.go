package mqtt

import (
	"errors"
	"fmt"
	"strings"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
)

const (
	defaultTopicPrefix          string = "redhat"
	controlMessageIncomingTopic string = "insights/#"
	controlMessageOutgoingTopic string = "insights/%s/control/in"
	dataMessageIncomingTopic    string = "insights/#"
	dataMessageOutgoingTopic    string = "insights/%s/data/in"
)

type TopicType int8

const (
	ControlTopicType TopicType = 0
	DataTopicType    TopicType = 1
)

type TopicVerifier struct {
	prefix string
}

func NewTopicVerifier(prefix string) *TopicVerifier {

	topicVerifier := &TopicVerifier{prefix: defaultTopicPrefix}
	if prefix != "" {
		topicVerifier.prefix = prefix
	}

	return topicVerifier
}

func (tv *TopicVerifier) VerifyIncomingTopic(topic string) (TopicType, domain.ClientID, error) {

	items := strings.Split(topic, "/")
	if len(items) != 5 {
		return ControlTopicType, "", errors.New("MQTT topic requires 4 sections: " + tv.prefix + ", insights, <clientID>, <type>, in " + topic)
	}

	if items[0] != tv.prefix || items[1] != "insights" || items[4] != "out" {
		return ControlTopicType, "", errors.New("MQTT topic needs to be " + tv.prefix + "/insights/<clientID>/<type>/out")
	}

	var topicType TopicType
	if items[3] == "control" {
		topicType = ControlTopicType
	} else if items[3] == "data" {
		topicType = DataTopicType
	} else {
		return ControlTopicType, "", errors.New("Invalid topic type")
	}

	return topicType, domain.ClientID(items[2]), nil
}

func NewTopicBuilder(prefix string) *TopicBuilder {

	topicBuilder := &TopicBuilder{prefix: defaultTopicPrefix}
	if prefix != "" {
		topicBuilder.prefix = prefix
	}

	return topicBuilder
}

type TopicBuilder struct {
	prefix string
}

func (tb *TopicBuilder) BuildOutgoingDataTopic(clientID domain.ClientID) string {
	topicStringFmt := tb.prefix + "/" + dataMessageOutgoingTopic
	topic := fmt.Sprintf(topicStringFmt, clientID)
	return topic
}

func (tb *TopicBuilder) BuildOutgoingControlTopic(clientID domain.ClientID) string {
	topicStringFmt := tb.prefix + "/" + controlMessageOutgoingTopic
	topic := fmt.Sprintf(topicStringFmt, clientID)
	return topic
}

func (tb *TopicBuilder) BuildIncomingWildcardDataTopic() string {
	return tb.prefix + "/" + dataMessageIncomingTopic
}

func (tb *TopicBuilder) BuildIncomingWildcardControlTopic() string {
	return tb.prefix + "/" + controlMessageIncomingTopic
}
