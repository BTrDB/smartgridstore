
#include <iostream>
#include <string>
#include <vector>
#include <boost/bind.hpp>
#include <boost/thread.hpp>
#include <boost/uuid/uuid_io.hpp>
#include "GSF/Common/Convert.h"
#include <GSF/Transport/Constants.h>
#include "GSF/Transport/DataSubscriber.h"
#include "types.hpp"

using namespace GSF::TimeSeries;
using namespace GSF::TimeSeries::Transport;
using namespace std;

std::mutex GEPDriver::driversMu;
map<DataSubscriber*, GEPDriver*> GEPDriver::ds2driver;
map<uint32_t, GEPDriver*> GEPDriver::id2driver;

void cbErrorMessage(DataSubscriber* source, const string& message);
void cbStatusMessage(DataSubscriber* source, const string& message);
void cbMetadata(DataSubscriber* source, const vector<uint8_t>& bytes);
void cbNewMeasurement(DataSubscriber* source, const vector<MeasurementPtr>& measurements);
void cbReconnect(DataSubscriber* source);
void cbConfigChanged(DataSubscriber* source);

GEPDriver::GEPDriver(uint64_t id, const char* host, uint16_t port, const char* expression)
{
  this->id = id;
  this->host = string(host);
  this->port = port;
  this->expression = string(expression);

  driversMu.lock();
  ds2driver[&this->subscriber] = this;
  id2driver[id] = this;
  driversMu.unlock();
}

GEPDriver::~GEPDriver()
{
  GEPDriver::driversMu.lock();
  GEPDriver::ds2driver.erase(&this->subscriber);
  GEPDriver::id2driver.erase(this->id);
  GEPDriver::driversMu.unlock();
  //Destructor of DataSubscriber will disconnect
}

void GEPDriver::SendRequestMetadata()
{
  subscriber.SendServerCommand(ServerCommand::MetadataRefresh);
}

bool GEPDriver::connect()
{
    return subscriber.GetSubscriberConnector().Connect(subscriber, info);
}

void GEPDriver::RequestMetadata(uint64_t id)
{
  GEPDriver *drv;
  GEPDriver::driversMu.lock();
  try
  {
    drv = GEPDriver::id2driver[id];
  }
  catch (const out_of_range& e)
  {
    //Already aborted. That's fine
    GEPDriver::driversMu.unlock();
    return;
  }
  GEPDriver::driversMu.unlock();
  drv->SendRequestMetadata();
}

void GEPDriver::Abort(uint64_t id)
{
  GEPDriver *drv;
  GEPDriver::driversMu.lock();
  try
  {
    drv = GEPDriver::id2driver[id];
  }
  catch (const out_of_range& e)
  {
    //Already aborted. That's fine
    GEPDriver::driversMu.unlock();
    return;
  }
  GEPDriver::driversMu.unlock();
  delete drv;
}

bool GEPDriver::Begin()
{
  SubscriberConnector& connector = subscriber.GetSubscriberConnector();
  connector.RegisterErrorMessageCallback(&cbErrorMessage);
  connector.RegisterReconnectCallback(&cbReconnect);
  connector.SetHostname(host);
  connector.SetPort(port);
  connector.SetMaxRetries(5);
  connector.SetRetryInterval(100);
  connector.SetAutoReconnect(true);
  info.FilterExpression = this->expression;
  subscriber.RegisterStatusMessageCallback(&cbStatusMessage);
  subscriber.RegisterErrorMessageCallback(&cbErrorMessage);
  subscriber.RegisterNewMeasurementsCallback(&cbNewMeasurement);
  subscriber.RegisterMetadataCallback(&cbMetadata);
  subscriber.RegisterConfigurationChangedCallback(&cbConfigChanged);
  if (connect())
  {
    ReSubscribe();
    return true;
  }
  else
  {
    return false;
  }
}

void GEPDriver::NewMeasurement(DataSubscriber* source, const vector<MeasurementPtr>& measurements) {
  measurement_t *downstream = new measurement_t [measurements.size()];
  string *sz = new string[measurements.size()];

  for (int i = 0; i < measurements.size(); i++)
  {
    downstream[i].ID = measurements[i]->ID;
    downstream[i].Source = measurements[i]->Source.c_str();
    sz[i] = boost::uuids::to_string(measurements[i]->SignalID);
    downstream[i].SignalID = sz[i].c_str();
    downstream[i].Tag = measurements[i]->Tag.c_str();
    downstream[i].Value = measurements[i]->AdjustedValue();
    downstream[i].Timestamp = ((measurements[i]->Timestamp - 621355968000000000)*100);
    downstream[i].Flags = measurements[i]->Flags;
  }
  downstreamMeasurementsCallback(id, downstream, measurements.size());
  delete [] sz;
  delete [] downstream;
}

void GEPDriver::Metadata(DataSubscriber* source, const vector<uint8_t>& bytes) {
  downstreamMetadataCallback(id, &bytes[0], bytes.size());
}

void GEPDriver::Message(DataSubscriber* source, bool isError, const string& message) {
  downstreamMessageCallback(id, isError, message.c_str());
}
void GEPDriver::ReSubscribe() {
  subscriber.Subscribe(info);
  SendRequestMetadata();
}

void GEPDriver::RegisterMeasurementsCallback(void (*cb) (uint64_t id, const measurement_t *meaurements, size_t count))
{
  this->downstreamMeasurementsCallback = cb;
}

void GEPDriver::RegisterMetadataCallback(void (*cb) (uint64_t id, const uint8_t* data, size_t count))
{
  this->downstreamMetadataCallback = cb;
}

void GEPDriver::RegisterMessageCallback(void (*cb) (uint64_t id, bool isError, const char* message))
{
  this->downstreamMessageCallback = cb;
}

void GEPDriver::RegisterConnectionFailed(void (*cb) (uint64_t id))
{
  this->downstreamConnectionFailed = cb;
}

void GEPDriver::ConnFailed()
{
  downstreamConnectionFailed(id);
}

void cbNewMeasurement(DataSubscriber* source, const vector<MeasurementPtr>& measurements)
{
  GEPDriver *drv;
  GEPDriver::driversMu.lock();
  try
  {
    drv = GEPDriver::ds2driver[source];
  }
  catch (const out_of_range& e)
  {
    cout << "fatal error: receiving data on unmapped driver" << endl;
    GEPDriver::driversMu.unlock();
    return;
  }
  GEPDriver::driversMu.unlock();
  drv->NewMeasurement(source, measurements);
}

void cbReconnect(DataSubscriber* source)
{
  GEPDriver *drv;
  GEPDriver::driversMu.lock();
  try
  {
    drv = GEPDriver::ds2driver[source];
  }
  catch (const out_of_range& e)
  {
    cout << "fatal error: receiving data on unmapped driver" << endl;
    GEPDriver::driversMu.unlock();
    return;
  }
  GEPDriver::driversMu.unlock();
  if (source->IsConnected())
  {
      drv->ReSubscribe();
  }
  else
  {
      drv->ConnFailed();
  }
}

void cbMetadata(DataSubscriber* source, const vector<uint8_t>& bytes)
{
  GEPDriver *drv;
  GEPDriver::driversMu.lock();
  try
  {
    drv = GEPDriver::ds2driver[source];
  }
  catch (const out_of_range& e)
  {
    cout << "fatal error: receiving metadata on unmapped driver" << endl;
    GEPDriver::driversMu.unlock();
    return;
  }
  GEPDriver::driversMu.unlock();
  drv->Metadata(source, bytes);
}



void cbStatusMessage(DataSubscriber* source, const string& message)
{
  GEPDriver *drv;
  GEPDriver::driversMu.lock();
  try
  {
    drv = GEPDriver::ds2driver[source];
  }
  catch (const out_of_range& e)
  {
    cout << "fatal error: receiving message on unmapped driver" << endl;
    GEPDriver::driversMu.unlock();
    return;
  }
  GEPDriver::driversMu.unlock();
  drv->Message(source, false, message);
}

void cbConfigChanged(DataSubscriber* source)
{
  cout << "got config changed" << endl;
}

void cbErrorMessage(DataSubscriber* source, const string& message)
{
  GEPDriver *drv;
  GEPDriver::driversMu.lock();
  try
  {
    drv = GEPDriver::ds2driver[source];
  }
  catch (const out_of_range& e)
  {
    cout << "fatal error: receiving message on unmapped driver" << endl;
    GEPDriver::driversMu.unlock();
    return;
  }
  GEPDriver::driversMu.unlock();
  drv->Message(source, true, message);
}
