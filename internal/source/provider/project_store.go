package provider

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

type Project struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	RepoURL           string `json:"repo_url"`
	GitHubUser        string `json:"github_user,omitempty"`
	RuntimeImage      string `json:"runtime_image,omitempty"`
	RuntimePullPolicy string `json:"runtime_pull_policy,omitempty"`
	SkillsImage       string `json:"skills_image,omitempty"`
	SkillsPullPolicy  string `json:"skills_pull_policy,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
	WorkflowStatus    string `json:"workflow_status,omitempty"`
	LastActionAt      string `json:"last_action_at,omitempty"`
}

type ProjectStore interface {
	ListProjects(ctx context.Context) ([]Project, error)
	GetProject(ctx context.Context, id string) (Project, bool, error)
	PutProject(ctx context.Context, project Project) error
	DeleteProject(ctx context.Context, id string) error
}

type fileProjectStore struct{}

func NewFileProjectStore() ProjectStore {
	return &fileProjectStore{}
}

func (s *fileProjectStore) ListProjects(ctx context.Context) ([]Project, error) {
	return []Project{}, nil
}

func (s *fileProjectStore) GetProject(ctx context.Context, id string) (Project, bool, error) {
	return Project{}, false, nil
}

func (s *fileProjectStore) PutProject(ctx context.Context, project Project) error {
	return nil // no-op for file backend for now
}

func (s *fileProjectStore) DeleteProject(ctx context.Context, id string) error {
	return nil // no-op for file backend for now
}

type ConfigMapProjectStore struct {
	client    kubernetes.Interface
	namespace string
	name      string
}

func NewConfigMapProjectStore(client kubernetes.Interface, namespace, name string) (*ConfigMapProjectStore, error) {
	if client == nil {
		return nil, errors.New("kubernetes client is required")
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil, errors.New("kubernetes namespace is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "smith-projects"
	}
	return &ConfigMapProjectStore{
		client:    client,
		namespace: namespace,
		name:      name,
	}, nil
}

func (s *ConfigMapProjectStore) ListProjects(ctx context.Context) ([]Project, error) {
	cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return []Project{}, nil
		}
		return nil, err
	}

	var projects []Project
	for _, data := range cm.Data {
		var p Project
		if err := json.Unmarshal([]byte(data), &p); err == nil {
			projects = append(projects, p)
		}
	}
	return projects, nil
}

func (s *ConfigMapProjectStore) GetProject(ctx context.Context, id string) (Project, bool, error) {
	id = strings.TrimSpace(id)
	cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return Project{}, false, nil
		}
		return Project{}, false, err
	}
	data, ok := cm.Data[id]
	if !ok {
		return Project{}, false, nil
	}
	var p Project
	if err := json.Unmarshal([]byte(data), &p); err != nil {
		return Project{}, false, err
	}
	return p, true, nil
}

func (s *ConfigMapProjectStore) PutProject(ctx context.Context, project Project) error {
	project.ID = strings.TrimSpace(project.ID)
	if project.ID == "" {
		return errors.New("project ID is required")
	}

	payload, err := json.Marshal(project)
	if err != nil {
		return err
	}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				// Create
				cm = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      s.name,
						Namespace: s.namespace,
					},
					Data: map[string]string{
						project.ID: string(payload),
					},
				}
				_, err = s.client.CoreV1().ConfigMaps(s.namespace).Create(ctx, cm, metav1.CreateOptions{})
				return err
			}
			return err
		}

		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cm.Data[project.ID] = string(payload)
		_, err = s.client.CoreV1().ConfigMaps(s.namespace).Update(ctx, cm, metav1.UpdateOptions{})
		return err
	})
}

func (s *ConfigMapProjectStore) DeleteProject(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if cm.Data == nil {
			return nil
		}
		if _, ok := cm.Data[id]; !ok {
			return nil
		}
		delete(cm.Data, id)
		_, err = s.client.CoreV1().ConfigMaps(s.namespace).Update(ctx, cm, metav1.UpdateOptions{})
		return err
	})
}
