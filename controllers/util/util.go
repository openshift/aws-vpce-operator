/*
Copyright 2022.

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

package util

import (
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
)

// DefaultAVOLogger returns a zap.Logger using RFC3339 timestamps for the vpcendpoint controller
func DefaultAVOLogger(controllerName string) (logr.Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)
	// TODO: Make this configurable
	// config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)

	zapBase, err := config.Build()
	if err != nil {
		return logr.Logger{}, err
	}

	logger := zapr.NewLogger(zapBase)
	return logger.WithName(controllerName), nil
}

// DefaultAVORateLimiter returns a rate limiter that reconciles more slowly than the default.
// The default is 5ms --> 1000s, but resources are created much more slowly in AWS than in
// Kubernetes, so this helps avoid AWS rate limits.
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits
func DefaultAVORateLimiter() workqueue.RateLimiter {
	return workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(1*time.Second, 5000*time.Second),
		// 10 qps, 100 bucket size, only for overall retry limiting (not per item)
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(10, 100)},
	)
}

// AWSEnvVarReadyzChecker is a healthz.Checker that returns an error if there are not enough environment variables
// set to create an AWS client for this operator to function.
func AWSEnvVarReadyzChecker(_ *http.Request) error {
	// These two in combination allow non-STS clusters to get the necessary credentials
	if _, ok := os.LookupEnv("AWS_SECRET_ACCESS_KEY"); ok {
		if _, ok := os.LookupEnv("AWS_ACCESS_KEY_ID"); ok {
			return nil
		}
	}

	// These two in combination allow STS clusters to get the necessary credentials
	if _, ok := os.LookupEnv("AWS_WEB_IDENTITY_TOKEN_FILE"); ok {
		if _, ok := os.LookupEnv("AWS_ROLE_ARN"); ok {
			return nil
		}
	}

	return errors.New("missing sufficient environment variables to build an AWS client")
}
