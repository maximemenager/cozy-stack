package intent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/cozy/cozy-stack/pkg/registry"
	"github.com/cozy/cozy-stack/pkg/utils"

	"github.com/cozy/cozy-stack/model/app"
	"github.com/cozy/cozy-stack/model/instance"
	"github.com/cozy/cozy-stack/pkg/consts"
	"github.com/cozy/cozy-stack/pkg/couchdb"
)

// Service is a struct for an app that can serve an intent
type Service struct {
	Slug string `json:"slug"`
	Href string `json:"href"`
}

// Intent is a struct for a call from a client-side app to have another app do
// something for it
type Intent struct {
	IID               string    `json:"_id,omitempty"`
	IRev              string    `json:"_rev,omitempty"`
	Action            string    `json:"action"`
	Type              string    `json:"type"`
	Permissions       []string  `json:"permissions"`
	Client            string    `json:"client"`
	Services          []Service `json:"services"`
	AvailableServices []string  `json:"available_services"`
}

// ID is used to implement the couchdb.Doc interface
func (in *Intent) ID() string { return in.IID }

// Rev is used to implement the couchdb.Doc interface
func (in *Intent) Rev() string { return in.IRev }

// DocType is used to implement the couchdb.Doc interface
func (in *Intent) DocType() string { return consts.Intents }

// Clone implements couchdb.Doc
func (in *Intent) Clone() couchdb.Doc {
	cloned := *in
	cloned.Permissions = make([]string, len(in.Permissions))
	copy(cloned.Permissions, in.Permissions)
	cloned.Services = make([]Service, len(in.Services))
	copy(cloned.Services, in.Services)
	return &cloned
}

// SetID is used to implement the couchdb.Doc interface
func (in *Intent) SetID(id string) { in.IID = id }

// SetRev is used to implement the couchdb.Doc interface
func (in *Intent) SetRev(rev string) { in.IRev = rev }

// Save will persist the intent in CouchDB
func (in *Intent) Save(instance *instance.Instance) error {
	if in.ID() != "" {
		return couchdb.UpdateDoc(instance, in)
	}
	return couchdb.CreateDoc(instance, in)
}

// GenerateHref creates the href where the service can be called for an intent
func (in *Intent) GenerateHref(instance *instance.Instance, slug, target string) string {
	u := instance.SubDomain(slug)
	parts := strings.SplitN(target, "#", 2)
	if len(parts[0]) > 0 {
		u.Path = parts[0]
	}
	if len(parts) == 2 && len(parts[1]) > 0 {
		u.Fragment = parts[1]
	}
	u.RawQuery = "intent=" + in.ID()
	return u.String()
}

// FillServices looks at all the application that can answer this intent
// and save them in the services field
func (in *Intent) FillServices(instance *instance.Instance) error {
	res, err := app.ListWebapps(instance)
	if err != nil {
		return err
	}
	for _, man := range res {
		if intent := man.FindIntent(in.Action, in.Type); intent != nil {
			href := in.GenerateHref(instance, man.Slug(), intent.Href)
			service := Service{Slug: man.Slug(), Href: href}
			in.Services = append(in.Services, service)
		}
	}
	return nil
}

type tmp struct {
	Data []*app.WebappManifest
}

// GetInstanceWebapps returns the list of available webapps for the instance
func GetInstanceWebapps(inst *instance.Instance) ([]string, error) {
	man := tmp{}
	apps := []string{}
	for _, regURL := range inst.Registries() {
		url := regURL.String() + "registry?filter[type]=webapp"
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		err = json.NewDecoder(res.Body).Decode(&man)
		if err != nil {
			return nil, err
		}

		for _, app := range man.Data {
			slug := app.Slug()
			if !utils.IsInArray(slug, apps) {
				apps = append(apps, slug)
			}
		}
	}

	return apps, nil
}

// FindAvailableServices finds services which can answer to the intent from non-installed
// instance webapps
func (in *Intent) FindAvailableServices(inst *instance.Instance) error {
	installedWebApps, err := app.ListWebapps(inst)
	if err != nil {
		return err
	}

	endSlugs := []string{}
	webapps, err := GetInstanceWebapps(inst)
	res := []*app.WebappManifest{}

	for _, wa := range webapps {
		found := false
		for _, iwa := range installedWebApps {
			if wa == iwa.Slug() {
				found = true
				break
			}
		}
		if !found {
			endSlugs = append(endSlugs, wa)
		}
	}

	for _, webapp := range endSlugs {
		webappMan := app.WebappManifest{}
		v, err := registry.GetLatestVersion(webapp, "stable", inst.Registries())
		if err != nil {
			continue
		}
		err = json.NewDecoder(bytes.NewReader(v.Manifest)).Decode(&webappMan)
		if err != nil {
			return err
		}
		res = append(res, &webappMan)
	}

	for _, man := range res {
		if intent := man.FindIntent(in.Action, in.Type); intent != nil {
			in.AvailableServices = append(in.AvailableServices, man.Slug())
		}
	}

	return nil
}

var _ couchdb.Doc = (*Intent)(nil)
