// Package linkheader parses the subset of RFC 8288 Link headers used by
// LTI Advantage services (AGS, NRPS) for pagination.
package linkheader

import "strings"

// Next returns the URL of the rel="next" link in header, or "" if there
// is none.
func Next(header string) string {
	if header == "" {
		return ""
	}
	for _, part := range strings.Split(header, ",") {
		segments := strings.Split(part, ";")
		if len(segments) < 2 {
			continue
		}
		urlPart := strings.TrimSpace(segments[0])
		if !strings.HasPrefix(urlPart, "<") || !strings.HasSuffix(urlPart, ">") {
			continue
		}
		for _, seg := range segments[1:] {
			seg = strings.TrimSpace(seg)
			if !strings.HasPrefix(seg, "rel=") {
				continue
			}
			if strings.Trim(strings.TrimPrefix(seg, "rel="), `"`) == "next" {
				return strings.Trim(urlPart, "<>")
			}
		}
	}
	return ""
}
