
#include "types.hpp"
#include "_cgo_export.h"

void cbMeasurement(uint64_t id, const measurement_t *measurements, size_t count);
void cbMetadata(uint64_t id, const uint8_t *data, size_t count);
void cbMessage(uint64_t id, bool isError, const char* message);
void cbFailed(uint64_t id);


uint8_t new_driver(uint64_t id, const char* host, uint16_t port, const char* expression)
{
  cout << "new_driver_called" << endl;
  GEPDriver *drv = new GEPDriver(id, host, port, expression);
  drv->RegisterMetadataCallback((void (*)(uint64_t, const uint8_t*, size_t))AdapterMetadata);
  drv->RegisterMessageCallback((void (*)(uint64_t, bool, const char*))AdapterMessage);
  drv->RegisterConnectionFailed(AdapterFailed);
  drv->RegisterMeasurementsCallback(AdapterMeasurement);
  drv->Begin();
}

measurement_t* measurement_at(measurement_t *mz, size_t index)
{
    return &mz[index];
}

void request_metadata(uint64_t id)
{
  GEPDriver::RequestMetadata(id);
}

void abort_driver(uint64_t id)
{
  GEPDriver::Abort(id);
}
