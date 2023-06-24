package statsd

import (
	sd "github.com/smira/go-statsd"
)

func getTags(labels map[string]string) []sd.Tag {
	tags := make([]sd.Tag, len(labels))

	for key, value := range labels {
		tags = append(tags, sd.StringTag(key, value))
	}

	return tags
}
