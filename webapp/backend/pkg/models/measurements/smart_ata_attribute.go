package measurements

import (
	"fmt"
	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
	"strconv"
	"strings"
)

type SmartAtaAttribute struct {
	AttributeId int    `json:"attribute_id"`
	Value       int64  `json:"value"`
	Threshold   int64  `json:"thresh"`
	Worst       int64  `json:"worst"`
	RawValue    int64  `json:"raw_value"`
	RawString   string `json:"raw_string"`
	WhenFailed  string `json:"when_failed"`

	//Generated data
	TransformedValue int64   `json:"transformed_value"`
	Status           int64   `json:"status"`
	StatusReason     string  `json:"status_reason,omitempty"`
	FailureRate      float64 `json:"failure_rate,omitempty"`
}

func (sa *SmartAtaAttribute) GetStatus() int64 {
	return sa.Status
}

func (sa *SmartAtaAttribute) Flatten() map[string]interface{} {

	idString := strconv.Itoa(sa.AttributeId)

	return map[string]interface{}{
		fmt.Sprintf("attr.%s.attribute_id", idString): idString,
		fmt.Sprintf("attr.%s.value", idString):        sa.Value,
		fmt.Sprintf("attr.%s.worst", idString):        sa.Worst,
		fmt.Sprintf("attr.%s.thresh", idString):       sa.Threshold,
		fmt.Sprintf("attr.%s.raw_value", idString):    sa.RawValue,
		fmt.Sprintf("attr.%s.raw_string", idString):   sa.RawString,
		fmt.Sprintf("attr.%s.when_failed", idString):  sa.WhenFailed,

		//Generated Data
		fmt.Sprintf("attr.%s.transformed_value", idString): sa.TransformedValue,
		fmt.Sprintf("attr.%s.status", idString):            sa.Status,
		fmt.Sprintf("attr.%s.status_reason", idString):     sa.StatusReason,
		fmt.Sprintf("attr.%s.failure_rate", idString):      sa.FailureRate,
	}
}
func (sa *SmartAtaAttribute) Inflate(key string, val interface{}) {
	if val == nil {
		return
	}
	keyParts := strings.Split(key, ".")

	switch keyParts[2] {
	case "attribute_id":
		attrId, err := strconv.Atoi(val.(string))
		if err == nil {
			sa.AttributeId = attrId
		}
	case "value":
		sa.Value = val.(int64)
	case "worst":
		sa.Worst = val.(int64)
	case "thresh":
		sa.Threshold = val.(int64)
	case "raw_value":
		sa.RawValue = val.(int64)
	case "raw_string":
		sa.RawString = val.(string)
	case "when_failed":
		sa.WhenFailed = val.(string)

	//generated
	case "transformed_value":
		sa.TransformedValue = val.(int64)
	case "status":
		sa.Status = val.(int64)
	case "status_reason":
		sa.StatusReason = val.(string)
	case "failure_rate":
		sa.FailureRate = val.(float64)

	}
}

//populate attribute status, using SMART Thresholds & Observed Metadata
// Chainable
func (sa *SmartAtaAttribute) PopulateAttributeStatus() *SmartAtaAttribute {
	if strings.ToUpper(sa.WhenFailed) == pkg.SmartWhenFailedFailingNow {
		//this attribute has previously failed
		sa.Status = pkg.SmartAttributeStatusFailed
		sa.StatusReason = "Attribute is failing manufacturer SMART threshold"
		//if the Smart Status is failed, we should exit early, no need to look at thresholds.
		return sa

	} else if strings.ToUpper(sa.WhenFailed) == pkg.SmartWhenFailedInThePast {
		sa.Status = pkg.SmartAttributeStatusWarning
		sa.StatusReason = "Attribute has previously failed manufacturer SMART threshold"
	}

	if smartMetadata, ok := thresholds.AtaMetadata[sa.AttributeId]; ok {
		sa.ValidateThreshold(smartMetadata)
	}

	return sa
}

// compare the attribute (raw, normalized, transformed) value to observed thresholds, and update status if necessary
func (sa *SmartAtaAttribute) ValidateThreshold(smartMetadata thresholds.AtaAttributeMetadata) {
	//TODO: multiple rules
	// try to predict the failure rates for observed thresholds that have 0 failure rate and error bars.
	// - if the attribute is critical
	//		- the failure rate is over 10 - set to failed
	//		- the attribute does not match any threshold, set to warn
	// - if the attribute is not critical
	//		- if failure rate is above 20 - set to failed
	// 		- if failure rate is above 10 but below 20 - set to warn

	//update the smart attribute status based on Observed thresholds.
	var value int64
	if smartMetadata.DisplayType == thresholds.AtaSmartAttributeDisplayTypeNormalized {
		value = int64(sa.Value)
	} else if smartMetadata.DisplayType == thresholds.AtaSmartAttributeDisplayTypeTransformed {
		value = sa.TransformedValue
	} else {
		value = sa.RawValue
	}

	for _, obsThresh := range smartMetadata.ObservedThresholds {

		//check if "value" is in this bucket
		if ((obsThresh.Low == obsThresh.High) && value == obsThresh.Low) ||
			(obsThresh.Low < value && value <= obsThresh.High) {
			sa.FailureRate = obsThresh.AnnualFailureRate

			if smartMetadata.Critical {
				if obsThresh.AnnualFailureRate >= 0.10 {
					sa.Status = pkg.SmartAttributeStatusFailed
					sa.StatusReason = "Observed Failure Rate for Critical Attribute is greater than 10%"
				}
			} else {
				if obsThresh.AnnualFailureRate >= 0.20 {
					sa.Status = pkg.SmartAttributeStatusFailed
					sa.StatusReason = "Observed Failure Rate for Attribute is greater than 20%"
				} else if obsThresh.AnnualFailureRate >= 0.10 {
					sa.Status = pkg.SmartAttributeStatusWarning
					sa.StatusReason = "Observed Failure Rate for Attribute is greater than 10%"
				}
			}

			//we've found the correct bucket, we can drop out of this loop
			return
		}
	}
	// no bucket found
	if smartMetadata.Critical {
		sa.Status = pkg.SmartAttributeStatusWarning
		sa.StatusReason = "Could not determine Observed Failure Rate for Critical Attribute"
	}

	return
}
