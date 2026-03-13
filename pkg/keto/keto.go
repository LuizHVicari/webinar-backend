package keto

import (
	"context"
	"net/http"

	ory "github.com/ory/client-go"
)

type Client struct {
	read  *ory.APIClient
	write *ory.APIClient
}

func New(readURL, writeURL string) *Client {
	readCfg := ory.NewConfiguration()
	readCfg.Servers = ory.ServerConfigurations{{URL: readURL}}

	writeCfg := ory.NewConfiguration()
	writeCfg.Servers = ory.ServerConfigurations{{URL: writeURL}}

	return &Client{
		read:  ory.NewAPIClient(readCfg),
		write: ory.NewAPIClient(writeCfg),
	}
}

func (c *Client) HasRelation(ctx context.Context, namespace, object, relation, subjectID string) (bool, error) {
	result, resp, err := c.read.PermissionAPI.CheckPermission(ctx).
		Namespace(namespace).
		Object(object).
		Relation(relation).
		SubjectId(subjectID).
		Execute()

	if err == nil {
		return result.Allowed, nil
	}
	if resp != nil && resp.StatusCode == http.StatusForbidden {
		return false, nil
	}
	return false, err
}

func (c *Client) AddRelation(ctx context.Context, namespace, object, relation, subjectID string) error {
	body := ory.CreateRelationshipBody{
		Namespace: &namespace,
		Object:    &object,
		Relation:  &relation,
		SubjectId: &subjectID,
	}
	_, _, err := c.write.RelationshipAPI.CreateRelationship(ctx).
		CreateRelationshipBody(body).
		Execute()
	return err
}

func (c *Client) DeleteRelation(ctx context.Context, namespace, object, relation, subjectID string) error {
	_, err := c.write.RelationshipAPI.DeleteRelationships(ctx).
		Namespace(namespace).
		Object(object).
		Relation(relation).
		SubjectId(subjectID).
		Execute()
	return err
}
