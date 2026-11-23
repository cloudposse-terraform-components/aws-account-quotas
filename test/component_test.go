package test

import (
	"context"
	"fmt"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	helper "github.com/cloudposse/test-helpers/pkg/atmos/component-helper"
	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/cloudposse/test-helpers/pkg/atmos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	quotaRegion      = "us-east-2"
	quotaServiceCode = "robomaker"
	quotaCode        = "L-D6554FB1"
)

// currentQuotaValue returns the quota value currently applied in the test
// account. Since AWS provider 6.9.0, aws_servicequotas_service_quota refuses
// to request a value below the current one, and the current value differs
// between accounts, so the test requests exactly the current value.
func currentQuotaValue(s *ComponentSuite) float64 {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(quotaRegion))
	require.NoError(s.T(), err)

	out, err := servicequotas.NewFromConfig(cfg).GetServiceQuota(context.Background(), &servicequotas.GetServiceQuotaInput{
		ServiceCode: awssdk.String(quotaServiceCode),
		QuotaCode:   awssdk.String(quotaCode),
	})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), out.Quota)
	require.NotNil(s.T(), out.Quota.Value)

	return *out.Quota.Value
}

type ComponentSuite struct {
	helper.TestSuite
}

type QuotaMetricDimension struct {
	Class    string `json:"class"`
	Resource string `json:"resource"`
	Service  string `json:"service"`
	Type     string `json:"type"`
}

type UsageMetric struct {
	MetricDimensions            []QuotaMetricDimension `json:"metric_dimensions"`
	MetricName                  string                 `json:"metric_name"`
	MetricNamespace             string                 `json:"metric_namespace"`
	MetricStatisticRecommendation string              `json:"metric_statistic_recommendation"`
}

type ServiceQuota struct {
	Adjustable    bool          `json:"adjustable"`
	Arn           string        `json:"arn"`
	DefaultValue  float64       `json:"default_value"`
	ID            string        `json:"id"`
	QuotaCode     string        `json:"quota_code"`
	QuotaName     string        `json:"quota_name"`
	RequestID     interface{}   `json:"request_id"`
	RequestStatus interface{}   `json:"request_status"`
	ServiceCode   string        `json:"service_code"`
	ServiceName   string        `json:"service_name"`
	UsageMetric   []UsageMetric `json:"usage_metric"`
	ValueReported float64       `json:"value reported (may be inaccurate)"`
	ValueRequested float64      `json:"value requested"`
}

func (s *ComponentSuite) TestBasic() {
	const component = "account-quotas/basic"
	const stack = "default-test"

	currentValue := currentQuotaValue(s)
	inputs := map[string]interface{}{
		"quotas": map[string]interface{}{
			"robomaker-applications": map[string]interface{}{
				"service_code": quotaServiceCode,
				"quota_name":   "Simulation applications",
				"value":        currentValue,
			},
		},
	}

	defer s.DestroyAtmosComponent(s.T(), component, stack, &inputs)
	options, _ := s.DeployAtmosComponent(s.T(), component, stack, &inputs)

	var quotas map[string]ServiceQuota
	atmos.OutputStruct(s.T(), options, "quotas", &quotas)

	accountID := aws.GetAccountId(s.T())

	assert.True(s.T(), quotas["robomaker-applications"].Adjustable)
	assert.Equal(s.T(), quotas["robomaker-applications"].Arn, fmt.Sprintf("arn:aws:servicequotas:us-east-2:%s:robomaker/L-D6554FB1", accountID))
	assert.Equal(s.T(), quotas["robomaker-applications"].DefaultValue, 40.0)
	assert.Equal(s.T(), quotas["robomaker-applications"].ID, "robomaker/L-D6554FB1")
	assert.Equal(s.T(), quotas["robomaker-applications"].QuotaCode, "L-D6554FB1")
	assert.Equal(s.T(), quotas["robomaker-applications"].QuotaName, "Simulation applications")
	assert.Nil(s.T(), quotas["robomaker-applications"].RequestID)
	assert.Nil(s.T(), quotas["robomaker-applications"].RequestStatus)
	assert.Equal(s.T(), quotas["robomaker-applications"].ServiceCode, "robomaker")
	assert.Equal(s.T(), quotas["robomaker-applications"].ServiceName, "AWS RoboMaker")
	assert.Len(s.T(), quotas["robomaker-applications"].UsageMetric, 1)
	assert.Equal(s.T(), quotas["robomaker-applications"].UsageMetric[0].MetricName, "ResourceCount")
	assert.Equal(s.T(), quotas["robomaker-applications"].UsageMetric[0].MetricNamespace, "AWS/Usage")
	assert.Equal(s.T(), quotas["robomaker-applications"].UsageMetric[0].MetricStatisticRecommendation, "Maximum")
	assert.NotNil(s.T(), quotas["robomaker-applications"].ValueReported)
	assert.Equal(s.T(), quotas["robomaker-applications"].ValueRequested, currentValue)

	s.DriftTest(component, stack, &inputs)
}

func (s *ComponentSuite) TestEnabledFlag() {
	const component = "account-quotas/disabled"
	const stack = "default-test"

	s.VerifyEnabledFlag(component, stack, nil)
}

func TestRunSuite(t *testing.T) {
	suite := new(ComponentSuite)
	helper.Run(t, suite)
}
