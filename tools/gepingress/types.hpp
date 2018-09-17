#ifndef __TYPES_HPP__
#define __TYPES_HPP__

#include <iostream>
#include <string>
#include <vector>
#include <boost/bind.hpp>
#include <boost/thread.hpp>
#include <boost/uuid/uuid_io.hpp>
#include "GSF/Common/Convert.h"
#include "GSF/Transport/DataSubscriber.h"

#include "GSF/Common/Convert.h"
#include "GSF/Transport/DataSubscriber.h"

extern "C" {
  #include "adapter.h"
}


using namespace GSF::TimeSeries;
using namespace GSF::TimeSeries::Transport;
using namespace std;



class GEPDriver
{
public:
  GEPDriver(uint64_t id, const char* host, uint16_t port, const char* expression);
  ~GEPDriver();
  bool Begin();
  void RegisterMeasurementsCallback(void (*cb) (uint64_t id, const measurement_t *meaurements, size_t count));
  void RegisterMetadataCallback(void (*cb) (uint64_t id, const uint8_t* data, size_t count));
  void RegisterMessageCallback(void (*cb) (uint64_t id, bool isError, const char* message));
  void RegisterConnectionFailed(void (*cb) (uint64_t id));
  void NewMeasurement(DataSubscriber* source, const vector<MeasurementPtr>& measurements);
  void Metadata(DataSubscriber* source, const vector<uint8_t>& bytes);
  void Message(DataSubscriber* source, bool isError, const string& message);
  void SendRequestMetadata();
  void ReSubscribe();
  void ConnFailed();
  bool connect();

private:
  void begin();
  uint64_t id;
  string host;
  uint16_t port;
  string expression;

  void (*downstreamMeasurementsCallback)(uint64_t id, const measurement_t *meaurements, size_t count);
  void (*downstreamMetadataCallback)(uint64_t id, const uint8_t* data, size_t count);
  void (*downstreamMessageCallback)(uint64_t id, bool isError, const char* message);
  void (*downstreamConnectionFailed)(uint64_t id);

  DataSubscriber subscriber;
  SubscriptionInfo info;

public:
  static void Abort(uint64_t id);
  static void RequestMetadata(uint64_t id);
  
  static std::mutex driversMu;
  static map<DataSubscriber*, GEPDriver*> ds2driver;
  static map<uint32_t, GEPDriver*> id2driver;
};

#endif
