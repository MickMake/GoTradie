package gobunnings

import "encoding/json"

type HateOASLink struct {
	Rel     string   `json:"rel,omitempty"`
	Href    string   `json:"href,omitempty"`
	Methods []string `json:"methods,omitempty"`
}

type EntryPoint struct {
	Meta  map[string]string `json:"_meta,omitempty"`
	Links []HateOASLink     `json:"_links,omitempty"`
}

func (e EntryPoint) Link(rel string) (HateOASLink, bool) { return FindLink(e.Links, rel) }

func FindLink(links []HateOASLink, rel string) (HateOASLink, bool) {
	for _, l := range links {
		if l.Rel == rel {
			return l, true
		}
	}
	return HateOASLink{}, false
}

type ProblemDetails struct {
	Type     string                     `json:"type,omitempty"`
	Title    string                     `json:"title,omitempty"`
	Status   int                        `json:"status,omitempty"`
	Detail   string                     `json:"detail,omitempty"`
	Instance string                     `json:"instance,omitempty"`
	Errors   map[string][]string        `json:"errors,omitempty"`
	Extra    map[string]json.RawMessage `json:"-"`
}

func (p *ProblemDetails) UnmarshalJSON(data []byte) error {
	type alias ProblemDetails
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(data, &raw)
	delete(raw, "type")
	delete(raw, "title")
	delete(raw, "status")
	delete(raw, "detail")
	delete(raw, "instance")
	delete(raw, "errors")
	*p = ProblemDetails(a)
	p.Extra = raw
	return nil
}

type RawObject map[string]any
