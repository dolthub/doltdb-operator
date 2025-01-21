/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
)

var _ = Describe("User Controller", func() {
	BeforeEach(func() {
		By("Waiting for DoltDB to be ready")
		expectReady(ctx, k8sClient, testDoltKey)
	})

	It("should reconcile", func() {
		userKey := types.NamespacedName{
			Name:      "dolt-app-user-test",
			Namespace: testDoltKey.Namespace,
		}

		user := doltv1alpha.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userKey.Name,
				Namespace: userKey.Namespace,
			},
			Spec: doltv1alpha.UserSpec{
				Name: userKey.Name,
				DoltDBRef: doltv1alpha.DoltDBRef{
					ObjectReference: doltv1alpha.ObjectReference{
						Name: testDoltKey.Name,
					},
				},
				PasswordSecretKeyRef: &doltv1alpha.SecretKeySelector{
					LocalObjectReference: doltv1alpha.LocalObjectReference{
						Name: testDoltAppUserPwdKey.Name,
					},
					Key: testDoltAppUserPwdSecretKey,
				},
			},
		}
		Expect(k8sClient.Create(ctx, &user)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
		})

		By("Expecting User to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, userKey, &user); err != nil {
				return false
			}
			return user.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting User to eventually have finalizer")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, userKey, &user); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(&user, dolt.UserFinalizerName)
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should update password", func() {
		var testDoltDB doltv1alpha.DoltDB

		By("Getting DoltDB")
		Expect(k8sClient.Get(ctx, testDoltKey, &testDoltDB)).To(Succeed())

		key := types.NamespacedName{
			Name:      "user-pwd-update",
			Namespace: testDoltKey.Namespace,
		}
		secretKey := testDoltAppUserPwdSecretKey

		By("Creating Secret")
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
				Labels: map[string]string{
					dolt.WatchLabel: "true",
				},
			},

			StringData: map[string]string{
				secretKey: "dolt!#123",
			},
		}
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())
		})

		By("Creating User")
		user := doltv1alpha.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: doltv1alpha.UserSpec{
				Name: "app-user-update",
				DoltDBRef: doltv1alpha.DoltDBRef{
					ObjectReference: doltv1alpha.ObjectReference{
						Name: testDoltKey.Name,
					},
				},
				PasswordSecretKeyRef: &doltv1alpha.SecretKeySelector{
					LocalObjectReference: doltv1alpha.LocalObjectReference{
						Name: key.Name,
					},
					Key: secretKey,
				},
			},
		}
		Expect(k8sClient.Create(ctx, &user)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
		})

		By("Expecting User to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, key, &user); err != nil {
				return false
			}
			return user.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting credentials to be valid")
		testSQLConnection(ctx, &testDoltDB, user.Username(), *user.Spec.PasswordSecretKeyRef)

		By("Updating password Secret")
		Eventually(func(g Gomega) bool {
			g.Expect(k8sClient.Get(ctx, key, &secret)).To(Succeed())
			secret.Data[secretKey] = []byte("NewPassword!#123")
			g.Expect(k8sClient.Update(ctx, &secret)).To(Succeed())
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting credentials to be valid after update")
		testSQLConnection(ctx, &testDoltDB, user.Username(), *user.Spec.PasswordSecretKeyRef)
	})
})
