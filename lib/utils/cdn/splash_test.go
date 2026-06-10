package cdn

import "testing"

func TestSplashThumbnailURL(t *testing.T) {
	tests := []struct {
		slug string
		want string
	}{
		{"vow", SplashBase + "/vow/tiny.jpg"},
		{"edp", SplashBase + "/edp/small.jpg"},
		{"pantheon", SplashBase + "/pantheon/small.png"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := SplashThumbnailURL(tt.slug); got != tt.want {
			t.Errorf("SplashThumbnailURL(%q) = %q, want %q", tt.slug, got, tt.want)
		}
	}
}

func TestActivitySplashThumbnailURL(t *testing.T) {
	tests := []struct {
		name     string
		isRaid   bool
		activity string
		version  string
		want     string
	}{
		{
			name:     "raid default",
			isRaid:   true,
			activity: "vow",
			want:     SplashBase + "/vow/tiny.jpg",
		},
		{
			name:     "edp epic",
			isRaid:   true,
			activity: "edp",
			want:     SplashBase + "/edp/small.jpg",
		},
		{
			name:     "pantheon boss without cdn asset",
			isRaid:   false,
			activity: "pantheon",
			version:  "morgeth-surpassing",
			want:     SplashBase + "/pantheon/small.png",
		},
		{
			name:     "pantheon generic",
			isRaid:   false,
			activity: "pantheon",
			want:     SplashBase + "/pantheon/small.png",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ActivitySplashThumbnailURL(tt.isRaid, tt.activity, tt.version)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
