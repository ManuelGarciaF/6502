#include <Arduino.h>
#include <SPI.h>

#define LATCH 10

inline void setAddress(uint8_t data);

void setup(void)
{
    pinMode(LATCH, OUTPUT);

    Serial.begin(250000);
    SPI.begin();

    setAddress(0);
}

void loop(void)
{
    if (Serial.available()) {
        uint8_t data = Serial.read();

        setAddress(data);
    }
}

// will be uint16_t once I plug the second SR
inline void setAddress(uint8_t data)
{
    SPI.beginTransaction(SPISettings(8000000, MSBFIRST, SPI_MODE0));
    SPI.transfer(data);
    SPI.endTransaction();

    digitalWrite(LATCH, HIGH);
    digitalWrite(LATCH, LOW);
}
