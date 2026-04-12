/**
 * @Author: lidonglin
 * @Description: Default Prometheus histogram upper bounds (milliseconds) for NewHistogramVec.
 * @File:  const.go
 * @Version: 1.0.0
 * @Date: 2022/11/03 10:34
 */

package tmetric

// defaultLatencyBuckets holds the default upper bounds, in milliseconds, for histograms
// created by NewHistogramVec.
var defaultLatencyBuckets = []float64{
	1.0, 2.0, 3.0, 4.0, 5.0,
	6.0, 8.0, 10.0, 13.0, 16.0,
	20.0, 25.0, 30.0, 40.0, 50.0,
	65.0, 80.0, 100.0, 130.0, 160.0,
	200.0, 250.0, 300.0, 400.0, 500.0,
	650.0, 800.0, 1000.0, 2000.0, 5000.0,
	10000.0, 20000.0, 50000.0, 100000.0,
}
