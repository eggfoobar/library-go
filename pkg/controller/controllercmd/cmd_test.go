package controllercmd

import (
	"context"
	"io/fs"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
)

type mockFile struct {
	fs.FS
	data []byte
}

func (m mockFile) Stat() (fs.FileInfo, error) { return nil, nil }
func (m mockFile) Read([]byte) (int, error)   { return 0, nil }
func (m mockFile) Close() error               { return nil }
func (m mockFile) ReadFile(fileName string) ([]byte, error) {
	return m.data, nil
}
func (m mockFile) Open(fileName string) (fs.File, error) {
	return m, nil
}

func newMockFile(content string) mockFile {
	return mockFile{
		data: []byte(content),
	}
}

func TestControllerCmdConfigLeaderElection(t *testing.T) {
	ver := version.Info{
		Major:    "0",
		Minor:    "1",
		Platform: "test",
	}
	typeMeta := metav1.TypeMeta{
		Kind:       "GenericOperatorConfig",
		APIVersion: "operator.openshift.io/v1alpha1",
	}

	testCases := []struct {
		desc                           string
		fileReader                     mockFile
		expected                       operatorv1alpha1.GenericOperatorConfig
		cmdConfigDisableLeaderElection bool
	}{
		{
			desc: "empty state, default config should be empty",
			fileReader: newMockFile(`{ 
				"apiVersion": "operator.openshift.io/v1alpha1", 
				"kind": "GenericOperatorConfig"
			}`),
			expected: operatorv1alpha1.GenericOperatorConfig{
				TypeMeta:       typeMeta,
				LeaderElection: configv1.LeaderElection{Disable: false},
			},
		},
		{
			desc: "leader election should be disabled from programatic cmd config",
			fileReader: newMockFile(`{ 
				"apiVersion": "operator.openshift.io/v1alpha1", 
				"kind": "GenericOperatorConfig"
			}`),
			expected: operatorv1alpha1.GenericOperatorConfig{
				TypeMeta:       typeMeta,
				LeaderElection: configv1.LeaderElection{Disable: true},
			},
			cmdConfigDisableLeaderElection: true,
		},
		{
			desc: "leader election should be disabled from GenericOperatorConfig",
			fileReader: newMockFile(`{ 
				"apiVersion": "operator.openshift.io/v1alpha1", 
				"kind": "GenericOperatorConfig",
				"leaderElection": {"disable": true}
			}`),
			expected: operatorv1alpha1.GenericOperatorConfig{
				TypeMeta:       typeMeta,
				LeaderElection: configv1.LeaderElection{Disable: true},
			},
		},
		{
			desc: "leader election disable should be superceded by programatic cmd config",
			fileReader: newMockFile(`{ 
				"apiVersion": "operator.openshift.io/v1alpha1", 
				"kind": "GenericOperatorConfig",
				"leaderElection": {"disable": false}
			}`),
			expected: operatorv1alpha1.GenericOperatorConfig{
				TypeMeta:       typeMeta,
				LeaderElection: configv1.LeaderElection{Disable: true},
			},
			cmdConfigDisableLeaderElection: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cmd := NewControllerCommandConfig(
				"test",
				ver,
				func(c context.Context, cc *ControllerContext) error { return nil })

			cmd.basicFlags.fileReader = tc.fileReader
			cmd.basicFlags.ConfigFile = "/some/config/path"
			cmd.DisableLeaderElection = tc.cmdConfigDisableLeaderElection

			unstructured, config, raw, err := cmd.Config()
			assert.Nilf(t, err, "err: %s", err)
			assert.NotNil(t, unstructured)
			assert.NotEmpty(t, raw)
			assert.Equal(t, &tc.expected, config)
		})
	}
}

func TestControllerCmdConfigBindAddress(t *testing.T) {
	ver := version.Info{
		Major:    "0",
		Minor:    "1",
		Platform: "test",
	}
	typeMeta := metav1.TypeMeta{
		Kind:       "GenericOperatorConfig",
		APIVersion: "operator.openshift.io/v1alpha1",
	}

	testCases := []struct {
		desc                 string
		fileReader           mockFile
		expected             operatorv1alpha1.GenericOperatorConfig
		cmdConfigBindAddress string
	}{
		{
			desc: "empty state, default config should be empty",
			fileReader: newMockFile(`{ 
				"apiVersion": "operator.openshift.io/v1alpha1", 
				"kind": "GenericOperatorConfig"
			}`),
			expected: operatorv1alpha1.GenericOperatorConfig{
				TypeMeta: typeMeta,
			},
		},
		{
			desc: "bind address should be configurable from GenericOperatorConfig",
			fileReader: newMockFile(`{ 
				"apiVersion": "operator.openshift.io/v1alpha1", 
				"kind": "GenericOperatorConfig",
				"servingInfo": {
					"bindAddress": "127.0.0.1"
				}
			}`),
			expected: operatorv1alpha1.GenericOperatorConfig{
				TypeMeta:       typeMeta,
				LeaderElection: configv1.LeaderElection{Disable: false},
				ServingInfo: configv1.HTTPServingInfo{
					ServingInfo: configv1.ServingInfo{
						BindAddress: "127.0.0.1",
					},
				},
			},
		},
		{
			desc: "bind address should be configurable from programatic cmd config",
			fileReader: newMockFile(`{ 
				"apiVersion": "operator.openshift.io/v1alpha1", 
				"kind": "GenericOperatorConfig"
			}`),
			expected: operatorv1alpha1.GenericOperatorConfig{
				TypeMeta:       typeMeta,
				LeaderElection: configv1.LeaderElection{Disable: false},
				ServingInfo: configv1.HTTPServingInfo{
					ServingInfo: configv1.ServingInfo{
						BindAddress: "127.0.0.1",
					},
				},
			},
			cmdConfigBindAddress: "127.0.0.1",
		},
		{
			desc: "bind address configuration from programatic cmd config should supersede GenericOperatorConfig",
			fileReader: newMockFile(`{ 
				"apiVersion": "operator.openshift.io/v1alpha1", 
				"kind": "GenericOperatorConfig",
				"servingInfo": {
					"bindAddress": "0.0.0.0"
				}
			}`),
			expected: operatorv1alpha1.GenericOperatorConfig{
				TypeMeta:       typeMeta,
				LeaderElection: configv1.LeaderElection{Disable: false},
				ServingInfo: configv1.HTTPServingInfo{
					ServingInfo: configv1.ServingInfo{
						BindAddress: "127.0.0.1",
					},
				},
			},
			cmdConfigBindAddress: "127.0.0.1",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cmd := NewControllerCommandConfig(
				"test",
				ver,
				func(c context.Context, cc *ControllerContext) error { return nil })

			cmd.basicFlags.fileReader = tc.fileReader
			cmd.basicFlags.ConfigFile = "/some/config/path"
			cmd.basicFlags.BindAddress = tc.cmdConfigBindAddress

			unstructured, config, raw, err := cmd.Config()
			assert.Nilf(t, err, "err: %s", err)
			assert.NotNil(t, unstructured)
			assert.NotEmpty(t, raw)
			assert.Equal(t, &tc.expected, config)
		})
	}
}
