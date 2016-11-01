// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package apiserver

import (
	"github.com/juju/errors"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/observer"
	"github.com/juju/juju/apiserver/params"
)

var RedirectError = &params.Error{Code: params.CodeRedirect, Message: "model migrated to new controller"}

// redirectAdminAPI is used to redirect user logins to the new
// controller for a migrated model.
type redirectAdminAPI struct {
	*admin

	redirectInfo params.RedirectInfoResult
}

func newRedirectAdminAPI(srv *Server, root *apiHandler, apiObserver observer.Observer, redirectInfo params.RedirectInfoResult) interface{} {
	return &redirectAdminAPI{
		admin: &admin{
			srv:         srv,
			root:        root,
			apiObserver: apiObserver,
		},
		redirectInfo: redirectInfo,
	}
}

// Admin returns an object that provides API access to methods that can be
// called even when not authenticated.
func (r *redirectAdminAPI) Admin(id string) (*redirectAdminAPI, error) {
	if id != "" {
		// Safeguard id for possible future use.
		return nil, common.ErrBadId
	}
	return r, nil
}

// Login returns a redirect for user logins, but allows machine logins
// as normal to allow workers to finish migration cleanup tasks.
func (a *redirectAdminAPI) Login(req params.LoginRequest) (params.LoginResult, error) {
	tagKind := names.UserTagKind
	if req.AuthTag != "" {
		var err error
		tagKind, err = names.TagKind(req.AuthTag)
		if err != nil {
			return params.LoginResult{}, errors.Trace(err)
		}
	}
	if tagKind == names.UserTagKind {
		return params.LoginResult{}, RedirectError
	}
	return a.login(req, 3)
}

// RedirectInfo returns redirected host information for the model.
func (a *redirectAdminAPI) RedirectInfo() (params.RedirectInfoResult, error) {
	return a.redirectInfo, nil
}
