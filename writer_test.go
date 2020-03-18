package agent

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
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
			Retryer:     aws.NoOpRetryer{},
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

func TestWriter_createGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCloudWatch := mock_cloudwatchlogsiface.NewMockClientAPI(ctrl)
	gomock.InOrder(
		mockCloudWatch.EXPECT().PutLogEventsRequest(gomock.Any()).Do(func(input *cloudwatchlogs.PutLogEventsInput) {
			assert.Equal(t, aws.StringValue(input.LogGroupName), testLogGroup, "Expected log group name to match")
			assert.Equal(t, aws.StringValue(input.LogStreamName), testLogStream, "Expected log stream name to match")
		}).Return(cloudwatchlogs.PutLogEventsRequest{
			Request: &aws.Request{
				Error:       awserr.New(cloudwatchlogs.ErrCodeResourceNotFoundException, "event stream not found", errors.New("api error")),
				HTTPRequest: &http.Request{},
				Retryer:     aws.NoOpRetryer{},
			},
		}),
		mockCloudWatch.EXPECT().CreateLogStreamRequest(gomock.Any()).Do(func(input *cloudwatchlogs.CreateLogStreamInput) {
			assert.Equal(t, aws.StringValue(input.LogGroupName), testLogGroup, "Expected log group name to match")
			assert.Equal(t, aws.StringValue(input.LogStreamName), testLogStream, "Expected log stream name to match")
		}).Return(cloudwatchlogs.CreateLogStreamRequest{
			Request: &aws.Request{
				Error:       awserr.New(cloudwatchlogs.ErrCodeResourceNotFoundException, "event group not found", errors.New("api error")),
				HTTPRequest: &http.Request{},
				Retryer:     aws.NoOpRetryer{},
			},
		}),
		mockCloudWatch.EXPECT().CreateLogGroupRequest(gomock.Any()).Do(func(input *cloudwatchlogs.CreateLogGroupInput) {
			assert.Equal(t, aws.StringValue(input.LogGroupName), testLogGroup, "Expected log group name to match")
		}).Return(cloudwatchlogs.CreateLogGroupRequest{
			Request: &aws.Request{
				Data:        &cloudwatchlogs.CreateLogGroupOutput{},
				HTTPRequest: &http.Request{},
				Retryer:     aws.NoOpRetryer{},
			},
		}),
		mockCloudWatch.EXPECT().CreateLogStreamRequest(gomock.Any()).Do(func(input *cloudwatchlogs.CreateLogStreamInput) {
			assert.Equal(t, aws.StringValue(input.LogGroupName), testLogGroup, "Expected log group name to match")
			assert.Equal(t, aws.StringValue(input.LogStreamName), testLogStream, "Expected log stream name to match")
		}).Return(cloudwatchlogs.CreateLogStreamRequest{
			Request: &aws.Request{
				Data:        &cloudwatchlogs.CreateLogStreamOutput{},
				HTTPRequest: &http.Request{},
				Retryer:     aws.NoOpRetryer{},
			},
		}),
		mockCloudWatch.EXPECT().PutLogEventsRequest(gomock.Any()).Do(func(input *cloudwatchlogs.PutLogEventsInput) {
			assert.Equal(t, aws.StringValue(input.LogEvents[0].Message), "hello", "Expected event message to match")
			assert.Equal(t, aws.StringValue(input.LogGroupName), testLogGroup, "Expected log group name to match")
			assert.Equal(t, aws.StringValue(input.LogStreamName), testLogStream, "Expected log stream name to match")
		}).Return(cloudwatchlogs.PutLogEventsRequest{
			Request: &aws.Request{
				Data:        &cloudwatchlogs.PutLogEventsOutput{},
				HTTPRequest: &http.Request{},
				Retryer:     aws.NoOpRetryer{},
			},
		}),
	)
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
