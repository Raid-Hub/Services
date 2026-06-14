package cdn

import (
	"fmt"
	"strings"
)

const SplashBase = "https://cdn.raidhub.io/content/splash"

// thumbnailFileOverrides lists CDN splash slugs that do not ship tiny.jpg.
var thumbnailFileOverrides = map[string]string{
	"edp":      "small.jpg",
	"pantheon": "small.png",
}

// SplashThumbnailURL returns a CDN splash image URL for Discord thumbnails.
func SplashThumbnailURL(slug string) string {
	slug = strings.Trim(slug, "/")
	if slug == "" {
		return ""
	}
	file, ok := thumbnailFileOverrides[slug]
	if !ok {
		file = "tiny.jpg"
	}
	return fmt.Sprintf("%s/%s/%s", SplashBase, slug, file)
}

// ActivitySplashThumbnailURL picks the splash slug for raids vs pantheon modes.
func ActivitySplashThumbnailURL(isRaid bool, activitySplashPath, versionPath string) string {
	if !isRaid {
		versionSlug := strings.Trim(versionPath, "/")
		if versionSlug != "" {
			return SplashThumbnailURL(versionSlug)
		}
		activitySlug := strings.Trim(activitySplashPath, "/")
		if activitySlug != "" {
			return SplashThumbnailURL(activitySlug)
		}
		return SplashThumbnailURL("pantheon")
	}
	return SplashThumbnailURL(activitySplashPath)
}
