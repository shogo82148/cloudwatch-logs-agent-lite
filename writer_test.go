package agent

import (
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/golang/mock/gomock"
	"github.com/shogo82148/cloudwatch-logs-agent-lite/mock_cloudwatchlogsiface"
	"github.com/stretchr/testify/assert"
)

const (
	testLogGroup  = "my-logs"
	testLogStream = "my-stream"
)

func TestWriter_WriteEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCloudWatch := mock_cloudwatchlogsiface.NewMockClientAPI(ctrl)
	mockCloudWatch.EXPECT().PutLogEventsRequest(gomock.Any()).Do(func(input *cloudwatchlogs.PutLogEventsInput) {
		assert.Equal(t, aws.StringValue(input.LogEvents[0].Message), "hello", "Expected event message to match")
		assert.Equal(t, aws.StringValue(input.LogGroupName), testLogGroup, "Expected log group name to match")
		assert.Equal(t, aws.StringValue(input.LogStreamName), testLogStream, "Expected log stream name to match")
	}).Return(cloudwatchlogs.PutLogEventsRequest{
		Request: &aws.Request{
			Data:        &cloudwatchlogs.PutLogEventsOutput{},
			HTTPRequest: &http.Request{},
		},
	})
	w := &Writer{
		LogGroupName:  testLogGroup,
		LogStreamName: testLogStream,
		logs:          mockCloudWatch,
	}
	n, err := w.WriteEvent(time.Now(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if n != len("hello") {
		t.Errorf("want %d, got %d", len("hello"), n)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
}
