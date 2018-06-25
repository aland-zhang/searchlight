package util

import (
	"encoding/json"
	"fmt"

	"github.com/appscode/kutil"
	api "github.com/appscode/searchlight/apis/monitoring/v1alpha1"
	cs "github.com/appscode/searchlight/client/clientset/versioned/typed/monitoring/v1alpha1"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/wait"
)

func CreateOrPatchIncident(c cs.MonitoringV1alpha1Interface, meta metav1.ObjectMeta, transform func(alert *api.Incident) *api.Incident) (*api.Incident, kutil.VerbType, error) {
	cur, err := c.Incidents(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		glog.V(3).Infof("Creating Incident %s/%s.", meta.Namespace, meta.Name)
		out, err := c.Incidents(meta.Namespace).Create(transform(&api.Incident{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Incident",
				APIVersion: api.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}))
		return out, kutil.VerbCreated, err
	} else if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	return PatchIncident(c, cur, transform)
}

func PatchIncident(c cs.MonitoringV1alpha1Interface, cur *api.Incident, transform func(*api.Incident) *api.Incident) (*api.Incident, kutil.VerbType, error) {
	return PatchIncidentObject(c, cur, transform(cur.DeepCopy()))
}

func PatchIncidentObject(c cs.MonitoringV1alpha1Interface, cur, mod *api.Incident) (*api.Incident, kutil.VerbType, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(curJson, modJson, curJson)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, kutil.VerbUnchanged, nil
	}
	glog.V(3).Infof("Patching Incident %s/%s with %s.", cur.Namespace, cur.Name, string(patch))
	out, err := c.Incidents(cur.Namespace).Patch(cur.Name, types.MergePatchType, patch)
	return out, kutil.VerbPatched, err
}

func TryUpdateIncident(c cs.MonitoringV1alpha1Interface, meta metav1.ObjectMeta, transform func(*api.Incident) *api.Incident) (result *api.Incident, err error) {
	attempt := 0
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.Incidents(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(e2) {
			return false, e2
		} else if e2 == nil {
			result, e2 = c.Incidents(cur.Namespace).Update(transform(cur.DeepCopy()))
			return e2 == nil, nil
		}
		glog.Errorf("Attempt %d failed to update Incident %s/%s due to %v.", attempt, cur.Namespace, cur.Name, e2)
		return false, nil
	})

	if err != nil {
		err = fmt.Errorf("failed to update Incident %s/%s after %d attempts due to %v", meta.Namespace, meta.Name, attempt, err)
	}
	return
}

func UpdateIncidentStatus(c cs.MonitoringV1alpha1Interface, cur *api.Incident, transform func(*api.IncidentStatus) *api.IncidentStatus, useSubresource ...bool) (*api.Incident, error) {
	if len(useSubresource) > 1 {
		return nil, errors.Errorf("invalid value passed for useSubresource: %v", useSubresource)
	}

	mod := &api.Incident{
		TypeMeta:   cur.TypeMeta,
		ObjectMeta: cur.ObjectMeta,
		Status:     *transform(cur.Status.DeepCopy()),
	}

	if len(useSubresource) == 1 && useSubresource[0] {
		return c.Incidents(cur.Namespace).UpdateStatus(mod)
	}

	out, _, err := PatchIncidentObject(c, cur, mod)
	return out, err
}