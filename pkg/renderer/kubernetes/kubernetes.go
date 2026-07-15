// Package kubernetes deterministically renders Mosaic graphs without a cluster client.
package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/kdihalas/mosaic/pkg/graph"
	"github.com/kdihalas/mosaic/pkg/renderer"
	"github.com/kdihalas/mosaic/pkg/value"
	"gopkg.in/yaml.v3"
	"regexp"
	"sort"
	"strings"
)

type Renderer struct{}

func New() *Renderer           { return &Renderer{} }
func (*Renderer) Name() string { return "kubernetes" }

type Input = renderer.RenderInput

var nameRE = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)
var keyRE = regexp.MustCompile(`^([A-Za-z0-9][-A-Za-z0-9_.]*/)?[A-Za-z0-9][-A-Za-z0-9_.]*$`)

type item struct {
	rank                int
	namespace, name, id string
	object              map[string]any
}

func (r *Renderer) Render(ctx context.Context, in renderer.RenderInput) (*renderer.ArtifactSet, diagnostics.List) {
	var ds diagnostics.List
	ns := "default"
	if x, ok := in.Options.Get("namespace"); ok {
		ns, _ = x.StringValue()
	}
	var items []item
	names := map[string]string{}
	for _, res := range in.Graph.List() {
		names[string(res.ID)] = res.Name
	}
	for _, res := range in.Graph.List() {
		select {
		case <-ctx.Done():
			return nil, append(ds, d("REN001", ctx.Err().Error(), res))
		default:
		}
		if !nameRE.MatchString(res.Name) {
			ds = append(ds, d("K8S001", "invalid Kubernetes name `"+res.Name+"`", res))
			continue
		}
		for k := range res.Labels {
			if !keyRE.MatchString(k) {
				ds = append(ds, d("K8S002", "invalid label key `"+k+"`", res))
			}
		}
		for k := range res.Annotations {
			if !keyRE.MatchString(k) {
				ds = append(ds, d("K8S003", "invalid annotation key `"+k+"`", res))
			}
		}
		o, rank, err := render(res, in.Environment, ns, names)
		if err != nil {
			ds = append(ds, d("K8S004", err.Error(), res))
			continue
		}
		items = append(items, item{rank, ns, res.Name, string(res.ID), o})
	}
	if ds.HasErrors() {
		return nil, ds
	}
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if a.namespace != b.namespace {
			return a.namespace < b.namespace
		}
		if a.name != b.name {
			return a.name < b.name
		}
		return a.id < b.id
	})
	objects := make([]any, len(items))
	var y bytes.Buffer
	for i, x := range items {
		objects[i] = x.object
		b, e := yaml.Marshal(x.object)
		if e != nil {
			return nil, append(ds, diagnostics.Diagnostic{Code: "K8S005", Severity: diagnostics.SeverityError, Message: e.Error()})
		}
		if i > 0 {
			y.WriteString("---\n")
		}
		y.Write(b)
	}
	j, _ := json.MarshalIndent(map[string]any{"apiVersion": "v1", "kind": "List", "items": objects}, "", "  ")
	j = append(j, '\n')
	return &renderer.ArtifactSet{Files: map[string][]byte{"kubernetes.yaml": y.Bytes(), "kubernetes.json": j}, Metadata: map[string]string{"renderer": "kubernetes"}}, ds
}
func render(r graph.Resource, env, ns string, names map[string]string) (map[string]any, int, error) {
	f := native(r.Fields)
	labels := clone(r.Labels)
	anns := clone(r.Annotations)
	anns["mosaic.dev/resource-id"] = string(r.ID)
	anns["mosaic.dev/environment"] = env
	if r.Metadata.Module != "" {
		anns["mosaic.dev/module"] = r.Metadata.Module
	}
	if r.Metadata.Source.SourceName != "" {
		anns["mosaic.dev/source"] = r.Metadata.Source.SourceName
	}
	meta := map[string]any{"name": r.Name, "namespace": ns, "labels": labels, "annotations": anns}
	switch r.Type {
	case "core.Config":
		return map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": meta, "data": f["data"]}, 2, nil
	case "core.ServiceAccount":
		return map[string]any{"apiVersion": "v1", "kind": "ServiceAccount", "metadata": meta}, 1, nil
	case "core.Exposure":
		ports := f["ports"]
		if ports == nil {
			ports = []any{map[string]any{"name": "http", "port": 80, "targetPort": 8080}}
		}
		return map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": meta, "spec": map[string]any{"selector": map[string]any{"app.kubernetes.io/name": targetName(f["workload"], names)}, "ports": ports}}, 3, nil
	case "core.Workload":
		rep := f["replicas"]
		if rep == nil {
			rep = json.Number("1")
		}
		container := map[string]any{"name": r.Name, "image": f["image"]}
		for _, k := range []string{"ports", "resources", "probes"} {
			if f[k] != nil {
				container[k] = f[k]
			}
		}
		if e := f["environment"]; e != nil {
			container["env"] = envList(e)
		}
		spec := map[string]any{"replicas": rep, "selector": map[string]any{"matchLabels": map[string]any{"app.kubernetes.io/name": r.Name}}, "template": map[string]any{"metadata": map[string]any{"labels": labels}, "spec": map[string]any{"containers": []any{container}}}}
		if sa := f["serviceAccount"]; sa != nil {
			spec["template"].(map[string]any)["spec"].(map[string]any)["serviceAccountName"] = targetName(sa, names)
		}
		return map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": meta, "spec": spec}, 4, nil
	case "core.Autoscaling":
		min := f["minimumReplicas"]
		max := f["maximumReplicas"]
		metrics := []any{}
		if cpu, ok := f["cpu"].(map[string]any); ok {
			metrics = append(metrics, map[string]any{"type": "Resource", "resource": map[string]any{"name": "cpu", "target": map[string]any{"type": "Utilization", "averageUtilization": cpu["averageUtilisation"]}}})
		}
		return map[string]any{"apiVersion": "autoscaling/v2", "kind": "HorizontalPodAutoscaler", "metadata": meta, "spec": map[string]any{"scaleTargetRef": map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "name": targetName(f["target"], names)}, "minReplicas": min, "maxReplicas": max, "metrics": metrics}}, 5, nil
	case "core.DisruptionProtection":
		spec := map[string]any{"selector": map[string]any{"matchLabels": map[string]any{"app.kubernetes.io/name": targetName(f["target"], names)}}}
		if f["minimumAvailable"] != nil {
			spec["minAvailable"] = f["minimumAvailable"]
		}
		if f["maximumUnavailable"] != nil {
			spec["maxUnavailable"] = f["maximumUnavailable"]
		}
		return map[string]any{"apiVersion": "policy/v1", "kind": "PodDisruptionBudget", "metadata": meta, "spec": spec}, 6, nil
	}
	return nil, 99, fmt.Errorf("unsupported resource type %s", r.Type)
}
func native(v value.Value) map[string]any {
	b, _ := v.CanonicalJSON()
	var x map[string]any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	_ = dec.Decode(&x)
	return x
}
func clone(m map[string]string) map[string]string {
	x := map[string]string{}
	for k, v := range m {
		x[k] = v
	}
	return x
}
func targetName(v any, names map[string]string) string {
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Sprint(v)
	}
	r, ok := m["$reference"].(map[string]any)
	if !ok {
		return ""
	}
	s, _ := r["target"].(string)
	if n, ok := names[s]; ok {
		return n
	}
	p := strings.Split(s, ".")
	if len(p) > 0 {
		return p[len(p)-1]
	}
	return s
}
func envList(v any) []any {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	x := make([]any, len(ks))
	for i, k := range ks {
		x[i] = map[string]any{"name": k, "value": m[k]}
	}
	return x
}
func d(code, msg string, r graph.Resource) diagnostics.Diagnostic {
	return diagnostics.Diagnostic{Code: code, Severity: diagnostics.SeverityError, Message: msg, Span: r.Metadata.Source}
}
