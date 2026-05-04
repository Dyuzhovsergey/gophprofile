package domain

import (
	"testing"
	"time"
)

func TestAvatar_IsDeleted(t *testing.T) {
	t.Run("not deleted", func(t *testing.T) {
		avatar := Avatar{}

		if avatar.IsDeleted() {
			t.Fatal("expected avatar to be not deleted")
		}
	})

	t.Run("deleted", func(t *testing.T) {
		deletedAt := time.Now()
		avatar := Avatar{
			DeletedAt: &deletedAt,
		}

		if !avatar.IsDeleted() {
			t.Fatal("expected avatar to be deleted")
		}
	})
}

func TestAvatar_IsOwner(t *testing.T) {
	tests := []struct {
		name   string
		avatar Avatar
		userID string
		want   bool
	}{
		{
			name: "same user",
			avatar: Avatar{
				UserID: "sergey",
			},
			userID: "sergey",
			want:   true,
		},
		{
			name: "different user",
			avatar: Avatar{
				UserID: "sergey",
			},
			userID: "ivan",
			want:   false,
		},
		{
			name: "empty user id",
			avatar: Avatar{
				UserID: "sergey",
			},
			userID: "",
			want:   false,
		},
		{
			name: "spaces user id",
			avatar: Avatar{
				UserID: "sergey",
			},
			userID: "   ",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.avatar.IsOwner(tt.userID); got != tt.want {
				t.Fatalf("unexpected result: got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestAvatar_HasThumbnail(t *testing.T) {
	tests := []struct {
		name   string
		avatar Avatar
		size   ThumbnailSize
		want   bool
	}{
		{
			name:   "nil thumbnails",
			avatar: Avatar{},
			size:   ThumbnailSize100,
			want:   false,
		},
		{
			name: "existing thumbnail",
			avatar: Avatar{
				ThumbnailS3Keys: map[ThumbnailSize]string{
					ThumbnailSize100: "thumbnails/avatar-id/100x100.jpg",
				},
			},
			size: ThumbnailSize100,
			want: true,
		},
		{
			name: "missing thumbnail",
			avatar: Avatar{
				ThumbnailS3Keys: map[ThumbnailSize]string{
					ThumbnailSize100: "thumbnails/avatar-id/100x100.jpg",
				},
			},
			size: ThumbnailSize300,
			want: false,
		},
		{
			name: "empty thumbnail key",
			avatar: Avatar{
				ThumbnailS3Keys: map[ThumbnailSize]string{
					ThumbnailSize100: "",
				},
			},
			size: ThumbnailSize100,
			want: false,
		},
		{
			name: "spaces thumbnail key",
			avatar: Avatar{
				ThumbnailS3Keys: map[ThumbnailSize]string{
					ThumbnailSize100: "   ",
				},
			},
			size: ThumbnailSize100,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.avatar.HasThumbnail(tt.size); got != tt.want {
				t.Fatalf("unexpected result: got %t, want %t", got, tt.want)
			}
		})
	}
}
