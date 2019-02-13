package e2e

import (
	goctx "context"
	"testing"
	"time"

	"github.com/3scale/3scale-operator/pkg/apis"
	apiv1alpha1 "github.com/3scale/3scale-operator/pkg/apis/api/v1alpha1"
	appsgroup "github.com/3scale/3scale-operator/pkg/apis/apps"
	appsv1alpha1 "github.com/3scale/3scale-operator/pkg/apis/apps/v1alpha1"
	"github.com/3scale/3scale-operator/pkg/controller/tenant"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	frameworke2eutil "github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clientappsv1 "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
)

func TestFullHappyPath(t *testing.T) {
	var err error

	apimanagerList := apiManagerList()
	tenantList := tenantList()

	err = framework.AddToFrameworkScheme(apis.AddToScheme, apimanagerList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	err = framework.AddToFrameworkScheme(apis.AddToScheme, tenantList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	err = ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		t.Fatalf("failed to initialize cluster resources: %v", err)
	}
	t.Log("initialized cluster resources")

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}
	f := framework.Global
	t.Log("waiting until operator Deployment is ready...")

	err = frameworke2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "3scale-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("operator Deployment is ready")

	// Deploy APIManager resource
	var productized bool = true
	apimanager := &appsv1alpha1.APIManager{
		Spec: appsv1alpha1.APIManagerSpec{
			AmpRelease:     "2.4",
			WildcardDomain: "test1.127.0.0.1.nip.io",
			Productized:    &productized,
			Evaluation:     true,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-apimanager",
			Namespace: namespace,
		},
	}

	err = f.Client.Create(goctx.TODO(), apimanager, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		t.Fatal(err)
	}

	osAppsV1Client, err := clientappsv1.NewForConfig(f.KubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	err = waitForAllApiManagerStandardDeploymentConfigs(t, f.KubeClient, osAppsV1Client, namespace, "3scale-operator", retryInterval, time.Minute*15)
	if err != nil {
		t.Fatal(err)
	}

	// Deploy Tenant resource
	// - Deploy AdminPass secret
	adminPassSecretName := "tenant01adminsecretname"
	adminPass := "thisisapass"
	adminPassSecret := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      adminPassSecretName,
			Labels:    map[string]string{"app": "3scale-operator"},
		},
		StringData: map[string]string{tenant.SecretAdminAccessTokenKey: adminPass},
		Type:       v1.SecretTypeOpaque,
	}
	err = f.Client.Create(goctx.TODO(), adminPassSecret, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Creating tenant admin pass secret")
	// TODO wait until secret is avaiable??

	// deploy tenant resource
	tenant := &apiv1alpha1.Tenant{
		Spec: apiv1alpha1.TenantSpec{
			UserName: "admin",
			Email:    "admin@example.com",
			OrgName:  "ECorp",
			AdminPasswordRef: v1.SecretReference{
				Name:      adminPassSecretName,
				Namespace: namespace,
			},
			MasterCredentialsRef: v1.SecretReference{
				Name:      adminPassSecretName,
				Namespace: namespace,
			},
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "tenant01",
			Namespace: namespace,
		},
	}
}

func tenantList() *apiv1alpha1.TenantList {
	return &apiv1alpha1.TenantList{
		TypeMeta: metav1.TypeMeta{
			Kind:       apiv1alpha1.TenantKind,
			APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
		},
	}
}

func apiManagerList() *appsv1alpha1.APIManagerList {
	return &appsv1alpha1.APIManagerList{
		TypeMeta: metav1.TypeMeta{
			Kind:       appsgroup.APIManagerKind,
			APIVersion: appsv1alpha1.SchemeGroupVersion.String(),
		},
	}
}
