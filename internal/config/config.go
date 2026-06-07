// Package config holds project-wide constants shared across tools.
package config

const (
	// OutputSuffix is appended to the stem of every optimized file.
	// Change this to rename all outputs at once across image and video tools.
	//   photo.png  → photo_optimized.webp
	//   clip.mp4   → clip_optimized.mp4
	OutputSuffix = "_optimized"
)
