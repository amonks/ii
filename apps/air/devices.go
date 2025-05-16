package main

import (
	"fmt"
	"strings"
)

// RoomID represents a unique identifier for a room
// These values are used directly in the database, so they must match the existing data
type RoomID string

// Defined room constants
const (
	RoomLivingRoom RoomID = "living room"
	RoomOffice     RoomID = "office"
	RoomKitchen    RoomID = "kitchen"
	RoomBedroom    RoomID = "bedroom"
)

// Room represents a physical location with monitoring devices
type Room struct {
	ID          RoomID // Identifier used in database and code
	DisplayName string // Human-readable name for display
}

// All monitored rooms
var Rooms = []Room{
	{ID: RoomLivingRoom, DisplayName: "Living Room"},
	{ID: RoomOffice, DisplayName: "Office"},
	{ID: RoomKitchen, DisplayName: "Kitchen"},
	{ID: RoomBedroom, DisplayName: "Bedroom"},
}

// DeviceType represents the type of device (Aranet4, Venta, etc.)
type DeviceType string

const (
	DeviceTypeAranet DeviceType = "Aranet4"
	DeviceTypeVenta  DeviceType = "Venta"
)

// ParametersByType defines all parameters that each device type can monitor
var ParametersByType = map[DeviceType][]string{
	DeviceTypeVenta:  {"temperature", "humidity", "dust", "water_level", "fan_rpm"},
	DeviceTypeAranet: {"temperature", "humidity", "co2", "pressure", "battery"},
}

// Device represents a single monitoring device
type Device struct {
	ID     string     // Unique identifier for the device (could be a MAC address or serial number)
	Type   DeviceType // Type of device (Aranet4, Venta, etc.)
	RoomID RoomID     // References Room.ID
}

// GetRoom returns the Room for this device
func (d Device) GetRoom() Room {
	for _, room := range Rooms {
		if room.ID == d.RoomID {
			return room
		}
	}
	// This should never happen with properly configured devices
	return Room{ID: d.RoomID, DisplayName: strings.Title(string(d.RoomID))}
}

// GetParameters returns the parameters for this device based on its type
func (d Device) GetParameters() []string {
	return ParametersByType[d.Type]
}

// GetDisplayName returns a human-readable name for this device
func (d Device) GetDisplayName() string {
	room := d.GetRoom()
	switch d.Type {
	case DeviceTypeAranet:
		return fmt.Sprintf("%s Aranet", room.DisplayName)
	case DeviceTypeVenta:
		return "Venta Air Purifier"
	default:
		return fmt.Sprintf("%s %s", room.DisplayName, d.Type)
	}
}

// All configured devices
var Devices = []Device{
	{
		ID:     "Aranet4 069F9",
		Type:   DeviceTypeAranet,
		RoomID: RoomLivingRoom,
	},
	{
		ID:     "Aranet4 0AC6E",
		Type:   DeviceTypeAranet,
		RoomID: RoomOffice,
	},
	{
		ID:     "Aranet4 09678",
		Type:   DeviceTypeAranet,
		RoomID: RoomKitchen,
	},
	{
		ID:     "Aranet4 0A007",
		Type:   DeviceTypeAranet,
		RoomID: RoomBedroom,
	},
	{
		ID:     "60:8A:10:B5:58:A0",
		Type:   DeviceTypeVenta,
		RoomID: RoomLivingRoom,
	},
}

// GetDevicesByRoom returns all devices in a specific room
func GetDevicesByRoom(roomID RoomID) []Device {
	var result []Device
	for _, device := range Devices {
		if device.RoomID == roomID {
			result = append(result, device)
		}
	}
	return result
}

// GetDeviceByID returns a device by its ID
func GetDeviceByID(id string) (Device, bool) {
	for _, device := range Devices {
		if device.ID == id {
			return device, true
		}
	}
	return Device{}, false
}

// GetRoomByID returns a room by its ID
func GetRoomByID(id RoomID) (Room, bool) {
	for _, room := range Rooms {
		if room.ID == id {
			return room, true
		}
	}
	return Room{}, false
}

// DeviceToRoom returns the room ID for a given device ID
func DeviceToRoom(deviceID string) (RoomID, bool) {
	device, found := GetDeviceByID(deviceID)
	if !found {
		return "", false
	}
	return device.RoomID, true
}

// GetDeviceDisplayNames returns a map of device IDs to display names
func GetDeviceDisplayNames() map[string]string {
	result := make(map[string]string)
	for _, device := range Devices {
		result[device.ID] = device.GetDisplayName()
	}
	return result
}

// GetRoomDisplayNames returns a map of room IDs to display names
// The keys match the exact strings stored in the database
func GetRoomDisplayNames() map[string]string {
	result := make(map[string]string)
	for _, room := range Rooms {
		result[string(room.ID)] = room.DisplayName
	}
	return result
}

