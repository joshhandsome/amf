package callback

import (
	"net/http"
	"testing"

	"github.com/h2non/gock"
	"github.com/stretchr/testify/require"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/openapi"
	"github.com/free5gc/openapi/models"
)

// Notifications must not need a working NRF or an OAuth token, even when OAuth is enabled.
func TestNotificationsWithoutOAuthToken(t *testing.T) {
	self := amf_context.GetSelf()
	oldOAuth, oldURI := self.OAuth2Required.Load(), self.NrfUri
	self.OAuth2Required.Store(true)
	self.NrfUri = "http://127.0.0.1:1"
	t.Cleanup(func() { self.OAuth2Required.Store(oldOAuth); self.NrfUri = oldURI })
	openapi.InterceptInnerHttp2Client(t, false)
	const endpoint = "http://127.0.0.20:8000"
	const callbackURI = endpoint + "/notifications"
	tests := []struct {
		name string
		send func(t *testing.T)
	}{
		{"AMF status", func(t *testing.T) {
			const id = "notify-oauth-regression"
			self.AMFStatusSubscriptions.Store(id, models.Amf_Comm_SubscriptionData{AmfStatusUri: callbackURI})
			t.Cleanup(func() { self.AMFStatusSubscriptions.Delete(id) })
			SendAmfStatusChangeNotify(string(models.Amf_Comm_StatusChange_UNAVAILABLE), nil)
		}},
		{"N1 transfer failure", func(t *testing.T) {
			ue := &amf_context.AmfUe{N1N2Message: &amf_context.N1N2Message{
				Request: models.N1N2MessageTransferRequestBody{JsonData: &models.Amf_Comm_N1N2MessageTransferReqData{
					N1n2FailureTxfNotifURI: callbackURI,
				}}, Status: models.Amf_Comm_N1N2MessageTransferCause_ATTEMPTING_TO_REACH_UE,
			}}
			SendN1N2TransferFailureNotification(ue, models.Amf_Comm_N1N2MessageTransferCause_UE_NOT_RESPONDING)
			require.Nil(t, ue.N1N2Message)
		}},
		{"N1 message", func(t *testing.T) {
			ue := &amf_context.AmfUe{}
			ue.N1N2MessageSubscription.Store(int64(1), models.Amf_Comm_UeN1N2InfoSubscriptionCreateData{
				N1NotifyCallbackUri: callbackURI, N1MessageClass: models.Amf_Comm_N1MessageClass_5_GMM,
			})
			SendN1MessageNotify(ue, models.Amf_Comm_N1MessageClass_5_GMM, []byte{1}, nil)
		}},
		{"N1 AMF reallocation", func(t *testing.T) {
			ue := &amf_context.AmfUe{TargetAmfProfile: &models.Nrf_NFDisc_NFProfile{
				DefaultNotificationSubscriptions: []models.Nrf_NFMgmt_DefaultNotificationSubscription{{
					NotificationType: models.Nrf_NFMgmt_NotificationType_N1_MESSAGES,
					N1MessageClass:   models.Amf_Comm_N1MessageClass_5_GMM, CallbackUri: callbackURI,
				}},
			}}
			require.NoError(t, SendN1MessageNotifyAtAMFReAllocation(ue, []byte{1}, nil))
		}},
		{"N2 information", func(t *testing.T) {
			ue := &amf_context.AmfUe{}
			ue.N1N2MessageSubscription.Store(int64(1), models.Amf_Comm_UeN1N2InfoSubscriptionCreateData{
				N2NotifyCallbackUri: callbackURI, N2InformationClass: models.Amf_Comm_N2InformationClass_SM,
			})
			SendN2InfoNotify(ue, models.Amf_Comm_N2InformationClass_SM, nil, []byte{1})
		}},
		{"N2 handover", func(t *testing.T) {
			require.NoError(t, SendN2InfoNotifyN2Handover(&amf_context.AmfUe{HandoverNotifyUri: callbackURI}, nil))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(gock.OffAll)
			gock.New(endpoint).Post("/notifications").AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
				require.Empty(t, req.Header.Get("Authorization"))
				return true, nil
			}).Reply(http.StatusNoContent)
			tt.send(t)
			require.True(t, gock.IsDone(), "notification was not sent")
		})
	}
}
