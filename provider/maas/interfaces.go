// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package maas

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/juju/errors"
	"github.com/juju/gomaasapi"

	"github.com/juju/juju/instance"
	"github.com/juju/juju/network"
)

////////////////////////////////////////////////////////////////////////////////
// TODO(dimitern): The types below should be part of gomaasapi.
// LKK Card: https://canonical.leankit.com/Boards/View/101652562/119310616

type maasLinkMode string

const (
	modeUnknown maasLinkMode = ""
	modeStatic  maasLinkMode = "static"
	modeDHCP    maasLinkMode = "dhcp"
	modeLinkUp  maasLinkMode = "link_up"
)

type maasInterfaceLink struct {
	ID_        int          `json:"id"`
	Subnet_    *maasSubnet  `json:"subnet,omitempty"`
	IPAddress_ string       `json:"ip_address,omitempty"`
	Mode_      maasLinkMode `json:"mode"`
}

// ID implements gomaasapi.Link
func (l *maasInterfaceLink) ID() int {
	return l.ID_
}

// Mode implements gomaasapi.Link
func (l *maasInterfaceLink) Mode() string {
	return string(l.Mode_)
}

// Subnet implements gomaasapi.Link
func (l *maasInterfaceLink) Subnet() gomaasapi.Subnet {
	return l.Subnet_
}

type maasInterfaceType string

const (
	typeUnknown  maasInterfaceType = ""
	typePhysical maasInterfaceType = "physical"
	typeVLAN     maasInterfaceType = "vlan"
	typeBond     maasInterfaceType = "bond"
)

type maasInterface struct {
	ID      int               `json:"id"`
	Name    string            `json:"name"`
	Type    maasInterfaceType `json:"type"`
	Enabled bool              `json:"enabled"`

	MACAddress  string   `json:"mac_address"`
	VLAN        maasVLAN `json:"vlan"`
	EffectveMTU int      `json:"effective_mtu"`

	Links []maasInterfaceLink `json:"links"`

	Parents  []string `json:"parents"`
	Children []string `json:"children"`

	ResourceURI string `json:"resource_uri"`
}

type maasVLAN struct {
	ID_         int    `json:"id"`
	Name_       string `json:"name"`
	VID_        int    `json:"vid"`
	MTU_        int    `json:"mtu"`
	Fabric_     string `json:"fabric"`
	ResourceURI string `json:"resource_uri"`
}

// ID implements gomaasapi.VLAN
func (v *maasVLAN) ID() int {
	return v.ID_
}

// Name implements gomaasapi.VLAN
func (v *maasVLAN) Name() string {
	return v.Name_
}

// Fabric implements gomaasapi.VLAN
func (v *maasVLAN) Fabric() string {
	return v.Fabric_
}

// VID implements gomaasapi.VLAN
func (v *maasVLAN) VID() int {
	return v.VID_
}

// MTU implements gomaasapi.VLAN
func (v *maasVLAN) MTU() int {
	return v.MTU_
}

// DHCP implements gomaasapi.VLAN
func (v *maasVLAN) DHCP() bool {
	// TODO (babbageclunk): check what this should be.
	return false
}

// PrimaryRack implements gomaasapi.VLAN
func (v *maasVLAN) PrimaryRack() string {
	// TODO (babbageclunk): check what this should be.
	return ""
}

// SecondaryRack implements gomaasapi.VLAN
func (v *maasVLAN) SecondaryRack() string {
	// TODO (babbageclunk): check what this should be.
	return ""
}

type maasSubnet struct {
	ID_         int      `json:"id"`
	Name_       string   `json:"name"`
	Space_      string   `json:"space"`
	VLAN_       maasVLAN `json:"vlan"`
	GatewayIP   string   `json:"gateway_ip"`
	DNSServers  []string `json:"dns_servers"`
	CIDR_       string   `json:"cidr"`
	ResourceURI string   `json:"resource_uri"`
}

// ID implements gomaasapi.Subnet
func (s *maasSubnet) ID() int {
	return s.ID_
}

// Name implements gomaasapi.Subnet
func (s *maasSubnet) Name() string {
	return s.Name_
}

// Space implements gomaasapi.Subnet
func (s *maasSubnet) Space() string {
	return s.Space_
}

// VLAN implements gomaasapi.Subnet
func (s *maasSubnet) VLAN() gomaasapi.VLAN {
	return &s.VLAN_
}

// Gateway implements gomaasapi.Subnet
func (s *maasSubnet) Gateway() string {
	return s.GatewayIP
}

// CIDR implements gomaasapi.Subnet
func (s *maasSubnet) CIDR() string {
	return s.CIDR_
}

func parseInterfaces(jsonBytes []byte) ([]maasInterface, error) {
	var interfaces []maasInterface
	if err := json.Unmarshal(jsonBytes, &interfaces); err != nil {
		return nil, errors.Annotate(err, "parsing interfaces")
	}
	return interfaces, nil
}

// maasObjectNetworkInterfaces implements environs.NetworkInterfaces() using the
// new (1.9+) MAAS API, parsing the node details JSON embedded into the given
// maasObject to extract all the relevant InterfaceInfo fields. It returns an
// error satisfying errors.IsNotSupported() if it cannot find the required
// "interface_set" node details field.
func maasObjectNetworkInterfaces(maasObject *gomaasapi.MAASObject, subnetsMap map[string]network.Id) ([]network.InterfaceInfo, error) {
	interfaceSet, ok := maasObject.GetMap()["interface_set"]
	if !ok || interfaceSet.IsNil() {
		// This means we're using an older MAAS API.
		return nil, errors.NotSupportedf("interface_set")
	}

	// TODO(dimitern): Change gomaasapi JSONObject to give access to the raw
	// JSON bytes directly, rather than having to do call MarshalJSON just so
	// the result can be unmarshaled from it.
	//
	// LKK Card: https://canonical.leankit.com/Boards/View/101652562/119311323

	rawBytes, err := interfaceSet.MarshalJSON()
	if err != nil {
		return nil, errors.Annotate(err, "cannot get interface_set JSON bytes")
	}

	interfaces, err := parseInterfaces(rawBytes)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return collectInterfaceInfo(interfaces, subnetsMap)
}

func (environ *maasEnviron) maas2NetworkInterfaces(instance maas2Instance, subnetsMap map[string]network.Id) ([]network.InterfaceInfo, error) {
	return collectInterfaceInfo(instance.machine.InterfaceSet(), subnetsMap)
}

func collectInterfaceInfo(interfaces []gomaasapi.Interface, subnetsMap map[string]network.Id) ([]network.InterfaceInfo, error) {
	infos := make([]network.InterfaceInfo, 0, len(interfaces))
	for i, iface := range interfaces {

		// The below works for all types except bonds and their members.
		parentName := strings.Join(iface.Parents(), "")
		var nicType network.InterfaceType
		switch maasInterfaceType(iface.Type()) {
		case typePhysical:
			nicType = network.EthernetInterface
			children := strings.Join(iface.Children(), "")
			if parentName == "" && len(iface.Children()) == 1 && strings.HasPrefix(children, "bond") {
				// FIXME: Verify the bond exists, regardless of its name.
				// This is a bond member, set the parent correctly (from
				// Juju's perspective) - to the bond itself.
				parentName = children
			}
		case typeBond:
			parentName = ""
			nicType = network.BondInterface
		case typeVLAN:
			nicType = network.VLAN_8021QInterface
		}

		nicInfo := network.InterfaceInfo{
			DeviceIndex:         i,
			MACAddress:          iface.MACAddress(),
			ProviderId:          network.Id(fmt.Sprintf("%v", iface.ID())),
			VLANTag:             iface.VLAN().VID(),
			InterfaceName:       iface.Name(),
			InterfaceType:       nicType,
			ParentInterfaceName: parentName,
			Disabled:            !iface.Enabled(),
			NoAutoStart:         !iface.Enabled(),
			// This is not needed anymore, but the provisioner still validates it's set.
			NetworkName: network.DefaultPrivate,
		}

		for _, link := range iface.Links() {
			switch maasLinkMode(link.Mode()) {
			case modeUnknown:
				nicInfo.ConfigType = network.ConfigUnknown
			case modeDHCP:
				nicInfo.ConfigType = network.ConfigDHCP
			case modeStatic, modeLinkUp:
				nicInfo.ConfigType = network.ConfigStatic
			default:
				nicInfo.ConfigType = network.ConfigManual
			}

			if link.IPAddress() == "" {
				logger.Debugf("interface %q has no address", iface.Name())
			} else {
				// We set it here initially without a space, just so we don't
				// lose it when we have no linked subnet below.
				nicInfo.Address = network.NewAddress(link.IPAddress())
				nicInfo.ProviderAddressId = network.Id(fmt.Sprintf("%v", link.ID()))
			}

			if link.Subnet() == nil {
				logger.Debugf("interface %q link %d missing subnet", iface.Name(), link.ID())
				infos = append(infos, nicInfo)
				continue
			}

			sub := link.Subnet()
			nicInfo.CIDR = sub.CIDR()
			nicInfo.ProviderSubnetId = network.Id(fmt.Sprintf("%v", sub.ID()))
			nicInfo.ProviderVLANId = network.Id(fmt.Sprintf("%v", sub.VLAN().ID()))

			// Now we know the subnet and space, we can update the address to
			// store the space with it.
			nicInfo.Address = network.NewAddressOnSpace(sub.Space(), link.IPAddress())
			spaceId, ok := subnetsMap[sub.CIDR()]
			if !ok {
				// The space we found is not recognised, no
				// provider id available.
				logger.Warningf("interface %q link %d has unrecognised space %q", iface.Name(), link.ID(), sub.Space())
			} else {
				nicInfo.Address.SpaceProviderId = spaceId
				nicInfo.ProviderSpaceId = spaceId
			}

			gwAddr := network.NewAddressOnSpace(sub.Space(), sub.Gateway())
			nicInfo.DNSServers = network.NewAddressesOnSpace(sub.Space(), sub.DNSServers()...)
			if ok {
				gwAddr.SpaceProviderId = spaceId
				for i := range nicInfo.DNSServers {
					nicInfo.DNSServers[i].SpaceProviderId = spaceId
				}
			}
			nicInfo.GatewayAddress = gwAddr
			nicInfo.MTU = sub.VLAN().MTU()

			// Each link we represent as a separate InterfaceInfo, but with the
			// same name and device index, just different addres, subnet, etc.
			infos = append(infos, nicInfo)
		}
	}
	return infos, nil
}

// NetworkInterfaces implements Environ.NetworkInterfaces.
func (environ *maasEnviron) NetworkInterfaces(instId instance.Id) ([]network.InterfaceInfo, error) {
	inst, err := environ.getInstance(instId)
	if err != nil {
		return nil, errors.Trace(err)
	}
	subnetsMap, err := environ.subnetToSpaceIds()
	if err != nil {
		return nil, errors.Trace(err)
	}
	mi := inst.(*maas1Instance)
	return maasObjectNetworkInterfaces(mi.maasObject, subnetsMap)
}
