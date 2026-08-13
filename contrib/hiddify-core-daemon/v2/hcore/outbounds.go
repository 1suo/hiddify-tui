package hcore

import (
	"fmt"

	hcommon "github.com/hiddify/hiddify-core/v2/hcommon"
	E "github.com/sagernet/sing/common/exceptions"
)

// ListOutboundGroups returns the current core-owned outbound groups. An idle
// daemon has no groups until a profile is started.
func ListOutboundGroups() *OutboundGroupList {
	static.lock.Lock()
	defer static.lock.Unlock()
	if static.Box() == nil || static.Context() == nil {
		return &OutboundGroupList{}
	}
	return static.GetAllProxiesInfo(nil, false)
}

func SelectCurrentOutbound(groupTag, outboundTag string) error {
	static.lock.Lock()
	defer static.lock.Unlock()
	if static.Box() == nil {
		return E.New("service not ready")
	}
	response, err := static.SelectOutbound(&SelectOutboundRequest{GroupTag: groupTag, OutboundTag: outboundTag})
	if err != nil {
		return err
	}
	if response.GetCode() != hcommon.ResponseCode_OK {
		return fmt.Errorf("select outbound: %s", response.GetMessage())
	}
	return nil
}

func TestCurrentOutbound(tag string) error {
	static.lock.Lock()
	defer static.lock.Unlock()
	if static.Box() == nil {
		return E.New("service not ready")
	}
	_, err := static.UrlTest(&UrlTestRequest{Tag: tag})
	return err
}
