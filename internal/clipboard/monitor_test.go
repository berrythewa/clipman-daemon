package clipboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"

	"github.com/berrythewa/clipman-daemon/internal/config"
	// "github.com/berrythewa/clipman-daemon/internal/broker"
	// "github.com/berrythewa/clipman-daemon/internal/storage"

	"github.com/berrythewa/clipman-daemon/pkg/utils"
    "github.com/berrythewa/clipman-daemon/internal/types"
    "github.com/berrythewa/clipman-daemon/pkg/compression"
    "github.com/berrythewa/clipman-daemon/internal/mocks"

)

func TestMonitor_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClip := NewMockClipboard(ctrl)
	mockStore := mocks.NewMockStorage(ctrl)
	mockMQ := mocks.NewMockMQTTClient(ctrl)

	// Updated to use []byte instead of string
	initialContent := &types.ClipboardContent{Data: []byte("initial content"), Type: types.TypeText}
	mockClip.EXPECT().Read().Return(initialContent, nil)
	mockStore.EXPECT().GetLatestContent().Return(nil, nil)
	mockStore.EXPECT().SaveContent(initialContent).Return(nil)
	mockMQ.EXPECT().PublishContent(initialContent).Return(nil)

	m := &Monitor{
		config:     &config.Config{PollingInterval: time.Millisecond * 10},
		mqttClient: mockMQ,
		logger:     utils.NewLogger(utils.LoggerOptions{Level: "debug"}),
		clipboard:  mockClip,
		storage:    mockStore,
		history:    NewClipboardHistory(10),
	}

	err := m.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Wait for the monitor to process initial content
	time.Sleep(time.Millisecond * 20)

	m.Stop()
}

func TestMonitor_ProcessNewContent(t *testing.T) {
	tests := []struct {
		name            string
		content         *types.ClipboardContent
		expectSave      bool
		expectPublish   bool
		expectTransform bool
		transformResult *types.ClipboardContent
	}{
		{
			name:          "Normal content",
			// Updated to use []byte instead of string
			content:       &types.ClipboardContent{Data: []byte("test content"), Type: types.TypeText},
			expectSave:    true,
			expectPublish: true,
		},
		{
			name:            "Content with transformation",
			// Updated to use []byte instead of string
			content:         &types.ClipboardContent{Data: []byte("TEST CONTENT"), Type: types.TypeText},
			expectSave:      true,
			expectPublish:   true,
			expectTransform: true,
			// Updated to use []byte instead of string
			transformResult: &types.ClipboardContent{Data: []byte("test content"), Type: types.TypeText},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockStorage(ctrl)
			mockMQ := mocks.NewMockMQTTClient(ctrl)

			m := &Monitor{
				config:     &config.Config{},
				mqttClient: mockMQ,
				logger:     utils.NewLogger(utils.LoggerOptions{Level: "debug"}),
				storage:    mockStore,
				history:    NewClipboardHistory(10),
				contentProcessor: NewContentProcessor(),
			}

			if tt.expectTransform {
				m.contentProcessor.AddTransformer(func(content *types.ClipboardContent) *types.ClipboardContent {
					return tt.transformResult
				})
			}

			if tt.expectSave {
				expectedContent := tt.content
				if tt.expectTransform {
					expectedContent = tt.transformResult
				}
				mockStore.EXPECT().SaveContent(expectedContent).Return(nil)
			}

			if tt.expectPublish {
				expectedContent := tt.content
				if tt.expectTransform {
					expectedContent = tt.transformResult
				}
				mockMQ.EXPECT().PublishContent(expectedContent).Return(nil)
			}

			m.processNewContent(tt.content)

			if tt.expectTransform {
				if diff := cmp.Diff(tt.transformResult, m.history.GetLast(1)[0].Content); diff != "" {
					t.Errorf("Transformed content mismatch (-want +got):\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tt.content, m.history.GetLast(1)[0].Content); diff != "" {
					t.Errorf("Content mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestMonitor_MonitorClipboard(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClip := NewMockClipboard(ctrl)
	mockStore := mocks.NewMockStorage(ctrl)
	mockMQ := mocks.NewMockMQTTClient(ctrl)

	// Updated to use []byte instead of string
	initialContent := &types.ClipboardContent{Data: []byte("initial content"), Type: types.TypeText}
	// Updated to use []byte instead of string
	newContent := &types.ClipboardContent{Data: []byte("new content"), Type: types.TypeText}

	mockClip.EXPECT().Read().Return(initialContent, nil).Times(1)
	mockClip.EXPECT().Read().Return(newContent, nil).Times(1)
	mockStore.EXPECT().SaveContent(initialContent).Return(nil)
	mockStore.EXPECT().SaveContent(newContent).Return(nil)
	mockMQ.EXPECT().PublishContent(initialContent).Return(nil)
	mockMQ.EXPECT().PublishContent(newContent).Return(nil)

	m := &Monitor{
		config:     &config.Config{PollingInterval: time.Millisecond * 10},
		mqttClient: mockMQ,
		logger:     utils.NewLogger(utils.LoggerOptions{Level: "debug"}),
		clipboard:  mockClip,
		storage:    mockStore,
		history:    NewClipboardHistory(10),
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel

	go m.monitorClipboard()

	// Wait for initial and new content to be processed
	time.Sleep(time.Millisecond * 25)

	m.Stop()

	if diff := cmp.Diff([]*types.ClipboardContent{newContent, initialContent}, m.history.GetLast(2)); diff != "" {
		t.Errorf("History mismatch (-want +got):\n%s", diff)
	}
}

func TestMonitor_ErrorHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClip := NewMockClipboard(ctrl)
	mockStore := mocks.NewMockStorage(ctrl)
	mockMQ := mocks.NewMockMQTTClient(ctrl)

	clipboardErr := errors.New("clipboard error")
	storageErr := errors.New("storage error")
	mqttErr := errors.New("MQTT error")

	mockClip.EXPECT().Read().Return(nil, clipboardErr).Times(1)
	mockClip.EXPECT().Read().Return(&types.ClipboardContent{Data: []byte("test"), Type: types.TypeText}, nil).Times(1)
	mockStore.EXPECT().SaveContent(gomock.Any()).Return(storageErr)
	mockMQ.EXPECT().PublishContent(gomock.Any()).Return(mqttErr)

	m := &Monitor{
		config:     &config.Config{PollingInterval: time.Millisecond * 10},
		mqttClient: mockMQ,
		logger:     utils.NewLogger(utils.LoggerOptions{Level: "debug"}),
		clipboard:  mockClip,
		storage:    mockStore,
		history:    NewClipboardHistory(10),
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel

	go m.monitorClipboard()

	// Wait for errors to be processed
	time.Sleep(time.Millisecond * 25)

	m.Stop()

	// Check that the monitor continued running despite errors
	if len(m.history.GetLast(1)) == 0 {
		t.Error("Monitor did not recover from errors")
	}
}

func BenchmarkMonitor_ProcessNewContent(b *testing.B) {
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()

	mockStore := mocks.NewMockStorage(ctrl)
	mockMQ := mocks.NewMockMQTTClient(ctrl)

	mockStore.EXPECT().SaveContent(gomock.Any()).Return(nil).AnyTimes()
	mockMQ.EXPECT().PublishContent(gomock.Any()).Return(nil).AnyTimes()

	m := &Monitor{
		config:     &config.Config{},
		mqttClient: mockMQ,
		logger:     utils.NewLogger(utils.LoggerOptions{Level: "debug"}),
		storage:    mockStore,
		history:    NewClipboardHistory(1000),
		contentProcessor: NewContentProcessor(),
	}

	content := &types.ClipboardContent{Data: []byte("test content"), Type: types.TypeText}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.processNewContent(content)
	}
}
