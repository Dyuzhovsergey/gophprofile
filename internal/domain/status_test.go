package domain

import "testing"

func TestUploadStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status UploadStatus
		want   bool
	}{
		{
			name:   "uploading is valid",
			status: UploadStatusUploading,
			want:   true,
		},
		{
			name:   "uploaded is valid",
			status: UploadStatusUploaded,
			want:   true,
		},
		{
			name:   "failed is valid",
			status: UploadStatusFailed,
			want:   true,
		},
		{
			name:   "unknown is invalid",
			status: UploadStatus("unknown"),
			want:   false,
		},
		{
			name:   "empty is invalid",
			status: UploadStatus(""),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Fatalf("unexpected result: got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestProcessingStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status ProcessingStatus
		want   bool
	}{
		{
			name:   "pending is valid",
			status: ProcessingStatusPending,
			want:   true,
		},
		{
			name:   "processing is valid",
			status: ProcessingStatusProcessing,
			want:   true,
		},
		{
			name:   "completed is valid",
			status: ProcessingStatusCompleted,
			want:   true,
		},
		{
			name:   "failed is valid",
			status: ProcessingStatusFailed,
			want:   true,
		},
		{
			name:   "unknown is invalid",
			status: ProcessingStatus("unknown"),
			want:   false,
		},
		{
			name:   "empty is invalid",
			status: ProcessingStatus(""),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Fatalf("unexpected result: got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestThumbnailSize_IsValid(t *testing.T) {
	tests := []struct {
		name string
		size ThumbnailSize
		want bool
	}{
		{
			name: "original is valid",
			size: ThumbnailSizeOriginal,
			want: true,
		},
		{
			name: "100x100 is valid",
			size: ThumbnailSize100,
			want: true,
		},
		{
			name: "300x300 is valid",
			size: ThumbnailSize300,
			want: true,
		},
		{
			name: "unknown is invalid",
			size: ThumbnailSize("500x500"),
			want: false,
		},
		{
			name: "empty is invalid",
			size: ThumbnailSize(""),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.size.IsValid(); got != tt.want {
				t.Fatalf("unexpected result: got %t, want %t", got, tt.want)
			}
		})
	}
}
