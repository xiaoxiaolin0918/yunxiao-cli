package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

const (
	envPackagesUsername     = "YUNXIAO_PACKAGES_USERNAME"
	envPackagesPassword     = "YUNXIAO_PACKAGES_PASSWORD"
	envPackagesProtocolBase = "YUNXIAO_PACKAGES_PROTOCOL_BASE"
	defaultPackagesProtocol = "https://packages.aliyun.com/api/protocol"
)

var packagesArtifactsUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload an artifact (Packages protocol; write)",
	Long: `Risk: write

Upload via Packages *protocol* API (Basic auth), not openapi-rdc JSON.

OpenAPI upload for GENERIC was announced (2025-02) but help docs are still
unpublished (products index only lists List/Get/Delete). Binary upload therefore
uses the repository-guide protocol host:

  GENERIC: POST {protocol}/{org}/generic/{repo}/files/{path}?version=...
           multipart field "file"  (same shape as Packages 仓库指南 / funpub)
  MAVEN:   PUT  {protocol}/{org}/maven/{repo}/{group}/{artifact}/{version}/{file}

Dual credentials (both required):
  1) OpenAPI identity — normal yunxiao auth (PAT / credentials / profile) for
     mustClient + ResolveOrgID (org segment in protocol URL).
  2) Packages protocol Basic — --username/--password or
     YUNXIAO_PACKAGES_USERNAME/PASSWORD from 仓库指南 (not the PAT).
     Required even with --dry-run (checked before HTTP).

--file must be a relative path under cwd that exists; dry-run still reads it.
Supported repo-type: GENERIC, MAVEN.
Not wrapped (use native clients):
  NPM → npm publish | DOCKER → docker push | NUGET → nuget/dotnet push | PYPI → twine

Use --dry-run to preview; --yes is accepted (same global flags as delete).`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repoID, _ := cmd.Flags().GetString("repo-id")
		repoType, _ := cmd.Flags().GetString("repo-type")
		file, _ := cmd.Flags().GetString("file")
		artifactPath, _ := cmd.Flags().GetString("path")
		version, _ := cmd.Flags().GetString("version")
		fileName, _ := cmd.Flags().GetString("file-name")
		description, _ := cmd.Flags().GetString("description")
		groupID, _ := cmd.Flags().GetString("group-id")
		artifactID, _ := cmd.Flags().GetString("artifact-id")
		classifier, _ := cmd.Flags().GetString("classifier")
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		protocolBase, _ := cmd.Flags().GetString("protocol-base")

		if err := requireFlags("repo-id", repoID, "repo-type", repoType, "file", file); err != nil {
			handleErr(err)
			return
		}
		repoType = strings.ToUpper(strings.TrimSpace(repoType))
		if username == "" {
			username = strings.TrimSpace(os.Getenv(envPackagesUsername))
		}
		if password == "" {
			password = os.Getenv(envPackagesPassword)
		}
		if protocolBase == "" {
			protocolBase = strings.TrimSpace(os.Getenv(envPackagesProtocolBase))
		}
		if protocolBase == "" {
			protocolBase = defaultPackagesProtocol
		}
		protocolBase = strings.TrimRight(protocolBase, "/")

		data, base, err := readRelativeFile(file)
		if err != nil {
			handleErr(err)
			return
		}
		if fileName == "" {
			fileName = base
		}

		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		orgID, err := c.ResolveOrgID(cmd.Context())
		if err != nil {
			handleErr(err)
			return
		}

		switch repoType {
		case "GENERIC":
			if err := requireFlags("path", artifactPath, "version", version); err != nil {
				handleErr(err)
				return
			}
			if username == "" || password == "" {
				handleErr(fmt.Errorf("GENERIC upload needs Packages protocol credentials: --username/--password (or %s/%s) from 仓库指南", envPackagesUsername, envPackagesPassword))
				return
			}
			fullURL, err := packagesGenericUploadURL(protocolBase, orgID, repoID, artifactPath, version, fileName, description)
			if err != nil {
				handleErr(err)
				return
			}
			preview := map[string]any{
				"method": "POST", "url": fullURL,
				"auth": "basic", "multipart": true, "file_field": "file",
				"filename": fileName, "size": len(data),
				"repo_type": "GENERIC", "path": strings.TrimLeft(artifactPath, "/"), "version": version,
			}
			handleErr(runMutating("packages artifacts upload", risk.Write, globalDryRun, globalYes, preview, func() error {
				var out any
				if err := c.PostMultipartURL(cmd.Context(), fullURL, "file", fileName, data, nil, username, password, &out); err != nil {
					return err
				}
				return output.Success(out, map[string]any{"risk": risk.Write, "repo_type": "GENERIC"})
			}))
		case "MAVEN":
			if err := requireFlags("group-id", groupID, "artifact-id", artifactID, "version", version); err != nil {
				handleErr(err)
				return
			}
			if username == "" || password == "" {
				handleErr(fmt.Errorf("MAVEN upload needs Packages protocol credentials: --username/--password (or %s/%s) from 仓库指南", envPackagesUsername, envPackagesPassword))
				return
			}
			fullURL, deployName, err := packagesMavenUploadURL(protocolBase, orgID, repoID, groupID, artifactID, version, classifier, fileName)
			if err != nil {
				handleErr(err)
				return
			}
			preview := map[string]any{
				"method": "PUT", "url": fullURL,
				"auth": "basic", "binary": true, "filename": deployName, "size": len(data),
				"repo_type": "MAVEN", "group_id": groupID, "artifact_id": artifactID, "version": version,
			}
			handleErr(runMutating("packages artifacts upload", risk.Write, globalDryRun, globalYes, preview, func() error {
				var out any
				if err := c.PutBinaryURL(cmd.Context(), fullURL, data, "application/octet-stream", username, password, &out); err != nil {
					return err
				}
				if out == nil {
					out = map[string]any{"uploaded": true, "url": fullURL, "filename": deployName}
				}
				return output.Success(out, map[string]any{"risk": risk.Write, "repo_type": "MAVEN"})
			}))
		case "NPM", "DOCKER", "NUGET", "PYPI":
			handleErr(fmt.Errorf("repo-type %s upload is not wrapped by CLI (use native client: NPM=npm publish, DOCKER=docker push, NUGET=nuget/dotnet push, PYPI=twine); OpenAPI binary upload docs are unpublished", repoType))
		default:
			handleErr(fmt.Errorf("unsupported --repo-type %q (supported: GENERIC, MAVEN; not wrapped: NPM, DOCKER, NUGET, PYPI)", repoType))
		}
	},
}

func packagesGenericUploadURL(protocolBase, orgID, repoID, artifactPath, version, fileName, description string) (string, error) {
	p := strings.TrimLeft(strings.TrimSpace(artifactPath), "/")
	if p == "" {
		return "", fmt.Errorf("--path must be non-empty (e.g. myproject/mypath)")
	}
	u, err := url.Parse(fmt.Sprintf("%s/%s/generic/%s/files/%s", protocolBase, url.PathEscape(orgID), url.PathEscape(repoID), pathEscapeSegments(p)))
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("version", version)
	if fileName != "" {
		q.Set("fileName", fileName)
	}
	if description != "" {
		q.Set("versionDescription", description)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func packagesMavenUploadURL(protocolBase, orgID, repoID, groupID, artifactID, version, classifier, fileName string) (string, string, error) {
	groupPath := strings.ReplaceAll(strings.TrimSpace(groupID), ".", "/")
	if groupPath == "" || artifactID == "" || version == "" {
		return "", "", fmt.Errorf("maven coordinates incomplete")
	}
	deployName := fileName
	if classifier != "" {
		ext := path.Ext(fileName)
		base := strings.TrimSuffix(path.Base(fileName), ext)
		// Prefer maven layout name when classifier set.
		deployName = fmt.Sprintf("%s-%s-%s%s", artifactID, version, classifier, ext)
		_ = base
	}
	full := fmt.Sprintf("%s/%s/maven/%s/%s/%s/%s/%s",
		protocolBase,
		url.PathEscape(orgID),
		url.PathEscape(repoID),
		pathEscapeSegments(groupPath),
		url.PathEscape(artifactID),
		url.PathEscape(version),
		url.PathEscape(deployName),
	)
	return full, deployName, nil
}

func pathEscapeSegments(p string) string {
	parts := strings.Split(p, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			continue
		}
		out = append(out, url.PathEscape(part))
	}
	return strings.Join(out, "/")
}

func init() {
	packagesArtifactsUploadCmd.Flags().String("repo-id", "", "package repository id (required)")
	packagesArtifactsUploadCmd.Flags().String("repo-type", "", "GENERIC or MAVEN (required)")
	packagesArtifactsUploadCmd.Flags().String("file", "", "relative file path under cwd (required)")
	packagesArtifactsUploadCmd.Flags().String("path", "", "GENERIC artifact path (required for GENERIC)")
	packagesArtifactsUploadCmd.Flags().String("version", "", "artifact version (required for GENERIC/MAVEN)")
	packagesArtifactsUploadCmd.Flags().String("file-name", "", "optional remote file name (default: local basename)")
	packagesArtifactsUploadCmd.Flags().String("description", "", "GENERIC optional version description")
	packagesArtifactsUploadCmd.Flags().String("group-id", "", "MAVEN groupId (required for MAVEN)")
	packagesArtifactsUploadCmd.Flags().String("artifact-id", "", "MAVEN artifactId (required for MAVEN)")
	packagesArtifactsUploadCmd.Flags().String("classifier", "", "MAVEN optional classifier")
	packagesArtifactsUploadCmd.Flags().String("username", "", "Packages protocol username (or "+envPackagesUsername+")")
	packagesArtifactsUploadCmd.Flags().String("password", "", "Packages protocol password (or "+envPackagesPassword+")")
	packagesArtifactsUploadCmd.Flags().String("protocol-base", "", "protocol base URL (default "+defaultPackagesProtocol+")")
}
