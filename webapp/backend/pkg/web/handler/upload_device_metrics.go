package handler

import (
	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/http"
)

func UploadDeviceMetrics(c *gin.Context) {
	//db := c.MustGet("DB").(*gorm.DB)
	logger := c.MustGet("LOGGER").(logrus.FieldLogger)
	appConfig := c.MustGet("CONFIG").(config.Interface)
	//influxWriteDb := c.MustGet("INFLUXDB_WRITE").(*api.WriteAPIBlocking)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	//appConfig := c.MustGet("CONFIG").(config.Interface)

	var collectorSmartData collector.SmartInfo
	err := c.BindJSON(&collectorSmartData)
	if err != nil {
		logger.Errorln("Cannot parse SMART data", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	//update the device information if necessary
	updatedDevice, err := deviceRepo.UpdateDevice(c, c.Param("wwn"), collectorSmartData)
	if err != nil {
		logger.Errorln("An error occurred while updating device data from smartctl metrics:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	// insert smart info
	smartData, err := deviceRepo.SaveSmartAttributes(c, c.Param("wwn"), collectorSmartData)
	if err != nil {
		logger.Errorln("An error occurred while saving smartctl metrics", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	if smartData.Status != pkg.DeviceStatusPassed {
		//there is a failure detected by Scrutiny, update the device status on the homepage.
		updatedDevice, err = deviceRepo.UpdateDeviceStatus(c, c.Param("wwn"), smartData.Status)
		if err != nil {
			logger.Errorln("An error occurred while updating device status", err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false})
			return
		}
	}

	// save smart temperature data (ignore failures)
	err = deviceRepo.SaveSmartTemperature(c, c.Param("wwn"), updatedDevice.DeviceProtocol, collectorSmartData)
	if err != nil {
		logger.Errorln("An error occurred while saving smartctl temp data", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	//check for error
	if updatedDevice.DeviceStatus != pkg.DeviceStatusPassed {
		//send notifications
		testNotify := notify.Notify{
			Config: appConfig,
			Payload: notify.Payload{
				FailureType:  notify.NotifyFailureTypeSmartFailure,
				DeviceName:   updatedDevice.DeviceName,
				DeviceType:   updatedDevice.DeviceProtocol,
				DeviceSerial: updatedDevice.SerialNumber,
				Test:         false,
			},
			Logger: logger,
		}
		_ = testNotify.Send() //we ignore error message when sending notifications.
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
