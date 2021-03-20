package kubernetes

import (
	"encoding/json"
	"strings"
)

type PatchSet []interface{}

type MetaObject interface {
	GetLabels() map[string]string
	GetAnnotations() map[string]string
}

type Patch struct {
	set         PatchSet
	annotations bool
	labels      bool
}

func NewPatch(set PatchSet, annotations bool, labels bool) *Patch {
	return &Patch{
		set:         set,
		annotations: annotations,
		labels:      labels,
	}
}

// Add a set of patches
func (p *Patch) Add(set PatchSet) {
	p.set = append(p.set, set...)
}

// Set a value
func (p *Patch) Set(path string, value string) {
	p.Add(p.getStringPatch(path, value))
}

// Remove a value
func (p *Patch) Remove(path string) {
	p.Add(p.getRemovePatch(path))
}

// SetLabel adds a patch set to set a label
func (p *Patch) SetLabel(o MetaObject, key string, value string) {
	p.Add(p.getLabelPatch(o, key, value))
}

// RemoveLabel adds a patch set to remove a label
func (p *Patch) RemoveLabel(key string) {
	p.Add(p.getRemovePatch("/metadata/labels/" + strings.Replace(key, "/", "~1", -1)))
}

// SetAnnotation adds a patch set to set an annotation
func (p *Patch) SetAnnotation(o MetaObject, key string, value string) {
	p.Add(p.getAnnotationPatch(o, key, value))
}

// RemoveAnnotation adds a patch set to remove a label
func (p *Patch) RemoveAnnotation(key string) {
	p.Add(p.getRemovePatch("/metadata/annotations/" + strings.Replace(key, "/", "~1", -1)))
}

func (p *Patch) getRemovePatch(path string) PatchSet {
	return []interface{}{patch{
		Op:   "remove",
		Path: path,
	}}
}

func (p *Patch) getStringPatch(path string, value string) PatchSet {
	return []interface{}{patch{
		Op:    "replace",
		Path:  path,
		Value: value,
	}}
}

func (p *Patch) getLabelPatch(o MetaObject, key string, value string) PatchSet {
	payload := []interface{}{}

	if !p.labels && (len(o.GetLabels())) == 0 {
		payload = append(payload, patchStruct{
			Op:   "add",
			Path: "/metadata/labels",
		})
	}

	p.labels = true

	payload = append(payload, patch{
		Op:    "replace",
		Path:  "/metadata/labels/" + strings.Replace(key, "/", "~1", -1),
		Value: value,
	})

	return payload
}

func (p *Patch) getAnnotationPatch(o MetaObject, key string, value string) PatchSet {
	payload := []interface{}{}

	if !p.annotations && (len(o.GetAnnotations())) == 0 {
		payload = append(payload, patchStruct{
			Op:   "add",
			Path: "/metadata/annotations",
		})
	}

	p.annotations = true

	payload = append(payload, patch{
		Op:    "replace",
		Path:  "/metadata/annotations/" + strings.Replace(key, "/", "~1", -1),
		Value: value,
	})

	return payload
}

func (p *Patch) String() string {
	data, _ := json.Marshal(p.set)
	return string(data)
}
