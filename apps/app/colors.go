package app

// ProxyColors are the colors available for proxy instances, matching the frontend colors.ts
var ProxyColors = []string{
	"#6d8086", // grey-light
	"#519aba", // blue
	"#8dc149", // green
	"#e37933", // orange
	"#f55385", // pink
	"#a074c4", // purple
	"#EE6167", // red
	"#d19a66", // lightyellow
	"#cbcb41", // yellow
	"#e5c07b", // chalky
	"#e06c75", // coral
	"#56b6c2", // cyan
	"#abb2bf", // ivory
	"#61afef", // malibu
	"#98c379", // sage
	"#c678dd", // violet
}

// NextProxyColor returns a color for a new proxy based on the current proxy count.
// Cycles through the list so colors repeat after all are used.
func NextProxyColor(existingCount int) string {
	return ProxyColors[existingCount%len(ProxyColors)]
}
