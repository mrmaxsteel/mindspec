package kernel

import "time"

// Now is the clock used for every kernel time comparison and durable timestamp.
var Now = time.Now
