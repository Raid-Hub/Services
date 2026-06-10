package difficultytier

import "testing"

func tierString(tier *string) string {
	if tier == nil {
		return ""
	}
	return *tier
}

func TestClassifyWithFeatSkulls(t *testing.T) {
	feats := map[uint32]struct{}{
		3123804375: {}, // Feat: Token Limit
	}

	tests := []struct {
		name   string
		skulls []uint32
		want   string
	}{
		{
			name:   "adventure",
			skulls: []uint32{skullAdventureTimeOnYourSide},
			want:   TierAdventure,
		},
		{
			name:   "custom with feat",
			skulls: []uint32{skullEmptyFeat, 3123804375},
			want:   TierCustom,
		},
		{
			name:   "standard empty custom",
			skulls: []uint32{skullEmptyFeat, 2673088233},
			want:   TierStandard,
		},
		{
			name:   "not tier collection",
			skulls: nil,
			want:   "",
		},
		{
			name:   "adventure wins over custom feat",
			skulls: []uint32{skullAdventureTimeOnYourSide, 3123804375},
			want:   TierAdventure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tierString(ClassifyWithFeatSkulls(tt.skulls, feats)); got != tt.want {
				t.Fatalf("ClassifyWithFeatSkulls() = %q, want %q", got, tt.want)
			}
		})
	}
}
