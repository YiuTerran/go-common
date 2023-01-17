package geoutil

import "math"

/**
  *  @author tryao
  *  @date 2022/12/30 11:21
**/

// GetDistance 返回两个经纬度之间的间距，单位是米
func GetDistance(lat1, lng1, lat2, lng2 float64) float64 {
	radius := 6371000.0 //6378137.0
	rad := math.Pi / 180.0
	lat1 = lat1 * rad
	lng1 = lng1 * rad
	lat2 = lat2 * rad
	lng2 = lng2 * rad
	theta := lng2 - lng1
	dist := math.Acos(math.Sin(lat1)*math.Sin(lat2) + math.Cos(lat1)*math.Cos(lat2)*math.Cos(theta))
	return dist * radius
}
