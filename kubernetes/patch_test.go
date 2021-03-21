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

	p := kubernetes.NewPatch(set)

	p.Set("key", "value")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"replace\",\"path\":\"key\",\"value\":\"value\"}]", txt)
}

func TestPatch_Remove(t *testing.T) {
	set := kubernetes.PatchSet{}

	p := kubernetes.NewPatch(set)

	p.Remove("key")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"remove\",\"path\":\"key\",\"value\":\"\"}]", txt)
}

func TestPatch_SetAnnotation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p := kubernetes.NewPatch()

	o := mock.NewMockMetaObject(ctrl)

	o.EXPECT().GetAnnotations().Return(nil)

	p.SetAnnotation(o, "key", "value")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"add\",\"path\":\"/metadata/annotations\",\"value\":{}},{\"op\":\"replace\",\"path\":\"/metadata/annotations/key\",\"value\":\"value\"}]", txt)
}

func TestPatch_SetAnnotation2(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p := kubernetes.NewPatch()

	o := mock.NewMockMetaObject(ctrl)
	o.EXPECT().GetAnnotations().Return(map[string]string{
		"test": "val1",
	})

	p.SetAnnotation(o, "key", "value")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"replace\",\"path\":\"/metadata/annotations/key\",\"value\":\"value\"}]", txt)
}

func TestPatch_SetAnnotation3(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p := kubernetes.NewPatch()

	o := mock.NewMockMetaObject(ctrl)
	o.EXPECT().GetAnnotations().Return(nil).AnyTimes()

	p.SetAnnotation(o, "key", "value")
	p.SetAnnotation(o, "key2", "value2")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"add\",\"path\":\"/metadata/annotations\",\"value\":{}},{\"op\":\"replace\",\"path\":\"/metadata/annotations/key\",\"value\":\"value\"},{\"op\":\"replace\",\"path\":\"/metadata/annotations/key2\",\"value\":\"value2\"}]", txt)
}

func TestPatch_RemoveAnnotation(t *testing.T) {
	p := kubernetes.NewPatch()
	p.RemoveAnnotation("key")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"remove\",\"path\":\"/metadata/annotations/key\",\"value\":\"\"}]", txt)
}

func TestPatch_SetLabel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p := kubernetes.NewPatch()

	o := mock.NewMockMetaObject(ctrl)
	o.EXPECT().GetLabels().Return(nil)

	p.SetLabel(o, "key", "value")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"add\",\"path\":\"/metadata/labels\",\"value\":{}},{\"op\":\"replace\",\"path\":\"/metadata/labels/key\",\"value\":\"value\"}]", txt)
}

func TestPatch_SetLabel2(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p := kubernetes.NewPatch()

	o := mock.NewMockMetaObject(ctrl)
	o.EXPECT().GetLabels().Return(map[string]string{
		"test": "val1",
	})

	p.SetLabel(o, "key", "value")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"replace\",\"path\":\"/metadata/labels/key\",\"value\":\"value\"}]", txt)
}

func TestPatch_SetLabel3(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p := kubernetes.NewPatch()

	o := mock.NewMockMetaObject(ctrl)
	o.EXPECT().GetLabels().Return(nil)

	p.SetLabel(o, "key1", "value1")
	p.SetLabel(o, "key2", "value2")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"add\",\"path\":\"/metadata/labels\",\"value\":{}},{\"op\":\"replace\",\"path\":\"/metadata/labels/key1\",\"value\":\"value1\"},{\"op\":\"replace\",\"path\":\"/metadata/labels/key2\",\"value\":\"value2\"}]", txt)
}

func TestPatch_RemoveLabel(t *testing.T) {

	set := kubernetes.PatchSet{}

	p := kubernetes.NewPatch(set)

	p.RemoveLabel("key")

	txt := p.String()

	assert.Equal(t, "[{\"op\":\"remove\",\"path\":\"/metadata/labels/key\",\"value\":\"\"}]", txt)
}
