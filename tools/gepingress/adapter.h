#ifndef __ADAPTER_H__
#define __ADAPTER_H__

#include <stdint.h>

typedef struct {
  // Identification number used in
  // human-readable measurement key.
  uint32_t ID;

  // Source used in human-
  // readable measurement key.
  const char* Source;

  // Measurement's globally
  // unique identifier.
  const char* SignalID;

  // Human-readable tag name to
  // help describe the measurement.
  const char* Tag;

  // Instantaneous value
  // of the measurement.
  double Value;

  // NOT THE SAME AS GSF TIMESTAMP
  // This is in unix nanoseconds since the epoch
  int64_t Timestamp;

  // Flags indicating the state of the measurement
  // as reported by the device that took it.
  uint32_t Flags;
} measurement_t;

typedef const measurement_t* measurement_arg;

uint8_t new_driver(uint64_t id, const char* host, uint16_t port, const char* expression);
measurement_t* measurement_at(measurement_t *mz, size_t index);
void abort_driver(uint64_t id);
void request_metadata(uint64_t id);

#endif
