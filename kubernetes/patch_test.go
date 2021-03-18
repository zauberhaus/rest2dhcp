package kubernetes_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/kubernetes"
	"github.com/zauberhaus/rest2dhcp/mock"
)

func TestPatch_Set(t *testing.T) {
	set := kubernetes.PatchSet{}

	p := kubernetes.NewPatch(set, false, false)

	p.Set("key", "value")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"replace\",\"path\":\"key\",\"value\":\"value\"}]", txt)
}

func TestPatch_Remove(t *testing.T) {
	set := kubernetes.PatchSet{}

	p := kubernetes.NewPatch(set, false, false)

	p.Remove("key")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"remove\",\"path\":\"key\",\"value\":\"\"}]", txt)
}

func TestPatch_SetAnnotation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	set := kubernetes.PatchSet{}

	p := kubernetes.NewPatch(set, false, false)

	o := mock.NewMockMetaObject(ctrl)

	o.EXPECT().GetAnnotations().Return(nil)

	p.SetAnnotation(o, "key", "value")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"add\",\"path\":\"/metadata/annotations\",\"value\":{}},{\"op\":\"replace\",\"path\":\"/metadata/annotations/key\",\"value\":\"value\"}]", txt)
}

func TestPatch_SetAnnotation2(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	set := kubernetes.PatchSet{}

	p := kubernetes.NewPatch(set, true, false)

	o := mock.NewMockMetaObject(ctrl)

	p.SetAnnotation(o, "key", "value")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"replace\",\"path\":\"/metadata/annotations/key\",\"value\":\"value\"}]", txt)
}

func TestPatch_RemoveAnnotation(t *testing.T) {

	set := kubernetes.PatchSet{}

	p := kubernetes.NewPatch(set, true, false)

	p.RemoveAnnotation("key")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"remove\",\"path\":\"/metadata/annotations/key\",\"value\":\"\"}]", txt)
}

func TestPatch_SetLabel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	set := kubernetes.PatchSet{}

	p := kubernetes.NewPatch(set, true, false)

	o := mock.NewMockMetaObject(ctrl)
	o.EXPECT().GetLabels().Return(nil)

	p.SetLabel(o, "key", "value")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"add\",\"path\":\"/metadata/labels\",\"value\":{}},{\"op\":\"replace\",\"path\":\"/metadata/labels/key\",\"value\":\"value\"}]", txt)
}

func TestPatch_SetLabel2(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	set := kubernetes.PatchSet{}

	p := kubernetes.NewPatch(set, false, true)

	o := mock.NewMockMetaObject(ctrl)

	p.SetLabel(o, "key", "value")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"replace\",\"path\":\"/metadata/labels/key\",\"value\":\"value\"}]", txt)
}

func TestPatch_RemoveLabel(t *testing.T) {

	set := kubernetes.PatchSet{}

	p := kubernetes.NewPatch(set, true, false)

	p.RemoveLabel("key")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"remove\",\"path\":\"/metadata/labels/key\",\"value\":\"\"}]", txt)
}
