package corehttp

import (
	"errors"
	"fmt"
	"net/http"
	gopath "path"
	"strings"

	files "github.com/ipfs/go-ipfs-files"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/tj/go-redirects"
	"github.com/ucarion/urlpath"
	"go.uber.org/zap"
)

func (i *gatewayHandler) handleRedirectsFile(w http.ResponseWriter, r *http.Request, redirectsFilePath ipath.Resolved, logger *zap.SugaredLogger) (bool, string, error) {
	// Convert the path into a file node
	node, err := i.api.Unixfs().Get(r.Context(), redirectsFilePath)
	if err != nil {
		return false, "", fmt.Errorf("could not get _redirects node: %v", err)
	}
	defer node.Close()

	// Convert the node into a file
	f, ok := node.(files.File)
	if !ok {
		return false, "", fmt.Errorf("could not convert _redirects node to file")
	}

	// Parse redirect rules from file
	redirectRules, err := redirects.Parse(f)
	if err != nil {
		return false, "", fmt.Errorf("could not parse redirect rules: %v", err)
	}
	logger.Debugf("redirectRules=%v", redirectRules)

	// Attempt to match a rule to the URL path, and perform the corresponding redirect or rewrite
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) > 3 {
		// All paths should start with /ipfs/cid/, so get the path after that
		urlPath := "/" + strings.Join(pathParts[3:], "/")
		rootPath := strings.Join(pathParts[:3], "/")
		// Trim off the trailing /
		urlPath = strings.TrimSuffix(urlPath, "/")

		logger.Debugf("_redirects: urlPath=", urlPath)
		for _, rule := range redirectRules {
			// get rule.From, trim trailing slash, ...
			fromPath := urlpath.New(strings.TrimSuffix(rule.From, "/"))
			logger.Debugf("_redirects: fromPath=%v", strings.TrimSuffix(rule.From, "/"))
			match, ok := fromPath.Match(urlPath)
			if !ok {
				continue
			}

			// We have a match!  Perform substitutions.
			toPath := rule.To
			toPath = replacePlaceholders(toPath, match)
			toPath = replaceSplat(toPath, match)

			logger.Debugf("_redirects: toPath=%v", toPath)

			// Rewrite
			if rule.Status == 200 {
				// Prepend the rootPath
				toPath = rootPath + rule.To
				return false, toPath, nil
			}

			// Or 404
			if rule.Status == 404 {
				toPath = rootPath + rule.To
				content404Path := ipath.New(toPath)
				err = i.serve404(w, r, content404Path)
				return true, toPath, err
			}

			// Or redirect
			http.Redirect(w, r, toPath, rule.Status)
			return true, toPath, nil
		}
	}

	// No redirects matched
	return false, "", nil
}

func replacePlaceholders(to string, match urlpath.Match) string {
	if len(match.Params) > 0 {
		for key, value := range match.Params {
			to = strings.ReplaceAll(to, ":"+key, value)
		}
	}

	return to
}

func replaceSplat(to string, match urlpath.Match) string {
	return strings.ReplaceAll(to, ":splat", match.Trailing)
}

// Returns a resolved path to the _redirects file located in the root CID path of the requested path
func (i *gatewayHandler) getRedirectsFile(r *http.Request) (ipath.Resolved, error) {
	// r.URL.Path is the full ipfs path to the requested resource,
	// regardless of whether path or subdomain resolution is used.
	rootPath, err := getRootPath(r.URL.Path)
	if err != nil {
		return nil, err
	}

	path := ipath.New(gopath.Join(rootPath, "_redirects"))
	resolvedPath, err := i.api.ResolvePath(r.Context(), path)
	if err != nil {
		return nil, err
	}
	return resolvedPath, nil
}

// Returns the root CID path for the given path
func getRootPath(path string) (string, error) {
	// Handle both ipfs and ipns paths
	//   /ipfs/CID
	//   /ipns/domain/ipfs/CID
	if strings.HasPrefix(path, ipfsPathPrefix) && strings.Count(gopath.Clean(path), "/") >= 2 {
		parts := strings.Split(path, "/")
		return gopath.Join(ipfsPathPrefix, parts[2]), nil
	} else if strings.HasPrefix(path, ipnsPathPrefix) && strings.Count(gopath.Clean(path), "/") >= 4 {
		parts := strings.Split(gopath.Clean(path), "/")
		// TODO(JJ): The path came in as /ipns/domain/ipfs/CID, but I'm returning /ipfs/CID.  Confirm this doesn't cause any issues.
		return gopath.Join(ipfsPathPrefix, parts[4]), nil
	} else {
		fmt.Printf("bad path=%v\n", path)
		return "", errors.New("failed to get root CID path")
	}
}

// TODO(JJ): Confirm approach
func hasOriginIsolation(r *http.Request) bool {
	_, gw := r.Context().Value("gw-hostname").(string)
	_, dnslink := r.Context().Value("dnslink-hostname").(string)

	// If dnslink, only proceed if has ipns path after the domain, such that we can get the root CID
	if dnslink {
		return isIpnsPathWithIpfsPath(r.URL.Path)
	}

	if gw {
		return true
	}

	return false
}

func isIpnsPathWithIpfsPath(path string) bool {
	if strings.HasPrefix(path, ipnsPathPrefix) && strings.Count(gopath.Clean(path), "/") >= 4 {
		parts := strings.Split(gopath.Clean(path), "/")
		return parts[3] == ipnsPathPrefix
	} else {
		return false
	}
}
