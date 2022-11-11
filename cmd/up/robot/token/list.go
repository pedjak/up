// Copyright 2022 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package token

import (
	"context"
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/robots"

	"github.com/upbound/up/internal/upbound"
)

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// listCmd creates a robot on Upbound.
type listCmd struct {
	RobotName string `arg:"" required:"" help:"Name of robot."`
}

// Run executes the list robot tokens command.
func (c *listCmd) Run(p pterm.TextPrinter, pt *pterm.TablePrinter, ac *accounts.Client, oc *organizations.Client, rc *robots.Client, upCtx *upbound.Context) error { //nolint:gocyclo
	a, err := ac.Get(context.Background(), upCtx.Account)
	if err != nil {
		return err
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}
	rs, err := oc.ListRobots(context.Background(), a.Organization.ID)
	if err != nil {
		return err
	}
	if len(rs) == 0 {
		return errors.Errorf(errFindRobotFmt, c.RobotName, upCtx.Account)
	}
	// TODO(hasheddan): because this API does not guarantee name uniqueness, we
	// must guarantee that exactly one robot exists in the specified account
	// with the provided name. Logic should be simplified when the API is
	// updated.
	var rid *uuid.UUID
	for _, r := range rs {
		if r.Name == c.RobotName {
			if rid != nil {
				return errors.Errorf(errMultipleRobotFmt, c.RobotName, upCtx.Account)
			}
			// Pin range variable so that we can take address.
			r := r
			rid = &r.ID
		}
	}
	if rid == nil {
		return errors.Errorf(errFindRobotFmt, c.RobotName, upCtx.Account)
	}

	ts, err := rc.ListTokens(context.Background(), *rid)
	if err != nil {
		return err
	}
	if len(ts.DataSet) == 0 {
		p.Printfln("No tokens found for robot %s in %s", c.RobotName, upCtx.Account)
		return nil
	}
	return printTokens(ts.DataSet, pt)
}

func printTokens(ts []common.DataSet, pt *pterm.TablePrinter) error {
	data := make([][]string, len(ts)+1)
	data[0] = []string{"NAME", "ID", "CREATED"}
	for i, t := range ts {
		n := fmt.Sprint(t.AttributeSet["name"])
		c := "n/a"
		if ca, ok := t.Meta["createdAt"]; ok {
			if ct, err := time.Parse(time.RFC3339, fmt.Sprint(ca)); err == nil {
				c = duration.HumanDuration(time.Since(ct))
			}
		}
		data[i+1] = []string{n, t.ID.String(), c}
	}
	return pt.WithHasHeader().WithData(data).Render()
}
