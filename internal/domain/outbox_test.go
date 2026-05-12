package domain

import "testing"

func TestOutboxEventType_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		eventType OutboxEventType
		want      bool
	}{
		{
			name:      "avatar uploaded",
			eventType: OutboxEventTypeAvatarUploaded,
			want:      true,
		},
		{
			name:      "avatar deleted",
			eventType: OutboxEventTypeAvatarDeleted,
			want:      true,
		},
		{
			name:      "unknown",
			eventType: OutboxEventType("unknown"),
			want:      false,
		},
		{
			name:      "empty",
			eventType: "",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.eventType.IsValid()
			if got != tt.want {
				t.Fatalf("unexpected result: got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOutboxEventStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status OutboxEventStatus
		want   bool
	}{
		{
			name:   "pending",
			status: OutboxEventStatusPending,
			want:   true,
		},
		{
			name:   "published",
			status: OutboxEventStatusPublished,
			want:   true,
		},
		{
			name:   "failed",
			status: OutboxEventStatusFailed,
			want:   true,
		},
		{
			name:   "unknown",
			status: OutboxEventStatus("unknown"),
			want:   false,
		},
		{
			name:   "empty",
			status: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.IsValid()
			if got != tt.want {
				t.Fatalf("unexpected result: got %v, want %v", got, tt.want)
			}
		})
	}
}
