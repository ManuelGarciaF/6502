#include <Arduino.h>
#include <SPI.h>

// 595 SRs are managed through SPI, only need to define the latch.
#define SR_LATCH_BIT PB2 // Part of PORTB

// The most significant bit in the 16 bits shifted out is wired to /OE.
#define OE_BIT_MASK 0x8000
#define ADDR_MASK 0x7FFF // 32k, A0-A14

// EEPROM data pins are D2-D9 -> I/0[0-7]. Map to PORTD[2:7] + PORTB[0:1]
#define DATA_D_MASK 0xFC // 0b11111100
#define DATA_B_MASK 0x03 // 0b00000011

#define WE_BIT PC0 // Part of PORTC

#define EEPROM_SIZE 0x8000 // 32k bytes
#define PAGE_SIZE 64       // Write page size in bytes

#define MAX_PAYLOAD 64 // bytes

#define MAGIC_HOST 0x42
#define MAGIC_DEVICE 0x24

#define WRITE_TIMEOUT_MS 20 // t_WC is max 10ms

// Structs
enum Code : uint8_t {
    CODE_READ = 0x01,
    CODE_WRITE = 0x02,
    CODE_ACK = 0x81,
    CODE_NAK = 0x82,
};

struct FrameHeader {
    uint8_t code;
    uint16_t addr;
    uint8_t len;
};

// Globals
static uint8_t payload[MAX_PAYLOAD];

// Static verification.
static_assert((ADDR_MASK & OE_BIT_MASK) == 0, "/OE must not overlap with the address");
static_assert((ADDR_MASK | OE_BIT_MASK) == 0xFFFF, "All SR outputs must be accounted for");

static_assert(__builtin_popcount(DATA_D_MASK) + __builtin_popcount(DATA_B_MASK) == 8, "Data bus must cover 8 pins");

static_assert((DATA_B_MASK & _BV(SR_LATCH_BIT)) == 0, "Data bus must not overlap SR latch");
static_assert((DATA_B_MASK & _BV(PB3)) == 0, "Data bus must not overlap MOSI");
static_assert((DATA_B_MASK & _BV(PB5)) == 0, "Data bus must not overlap SCK");

static_assert(MAX_PAYLOAD >= PAGE_SIZE, "Payload buffer must hold a full page");

/*
** Function definitions
*/
static void sendNak(uint16_t addr);
static void handleRead(const FrameHeader *header, uint8_t bytesToRead);
static void handleWrite(const FrameHeader *header, const uint8_t *data);
static void sendFrame(const FrameHeader *header, const uint8_t *data);
static uint8_t frameCheckSum(uint8_t magic, const FrameHeader *header, const uint8_t *data);
static uint8_t readByte(uint16_t addr);
static bool readPage(uint16_t baseAddr, uint8_t *buffer, uint8_t len);
static void loadByte(uint16_t addr, uint8_t data);
static bool writePage(uint16_t baseAddr, const uint8_t *data, uint8_t len);
static bool waitForWrite(uint16_t addr, uint8_t expected);
static void busToEeprom(uint16_t addr);
static void busToArduino(uint16_t addr, uint8_t data);
static void setAddress(uint16_t addr, bool outputEnable);
static void busOut(void);
static void busIn(void);
static void busWrite(uint8_t data);
static uint8_t readBus(void);

/*
** Main functions
*/
void setup(void)
{
    DDRB |= _BV(SR_LATCH_BIT); // Set latch pin to output.

    // Set /WE pin to output, ensure it starts off high
    // to avoid accidental writes on power on.
    PORTC |= _BV(WE_BIT);
    DDRC |= _BV(WE_BIT);

    Serial.begin(1000000);
    Serial.setTimeout(100); // Lower timeout to 100ms.

    SPI.begin();

    PORTB &= ~_BV(SR_LATCH_BIT); // Set latch low just in case.

    /* END OF SETUP */
    busToArduino(0x0000, 0x00); // Leave bus in a safe state.
}

void loop(void)
{
    // Wait for a frame to appear on the serial connection and answer.

    // Wait for the magic bit to appear. Serial.read returns -1 when no data is available.
    if (Serial.read() != MAGIC_HOST) {
        return;
    }

    // Read header.
    uint8_t headerBytes[4];
    if (Serial.readBytes(headerBytes, sizeof(headerBytes)) != sizeof(headerBytes)) {
        return; // Timed out mid-read.
    }

    FrameHeader header;
    header.code = headerBytes[0];
    header.addr = (uint16_t)headerBytes[1] | ((uint16_t)headerBytes[2] << 8); // Little endian encoded.
    header.len = headerBytes[3];

    if (header.len > MAX_PAYLOAD) {
        sendNak(header.addr);
        return;
    }

    // Read the payload. It lives in a global buffer.
    if (Serial.readBytes(payload, header.len) != header.len) {
        return; // Timed out mid-read
    }

    uint8_t checksum;
    if (Serial.readBytes(&checksum, 1) != 1) {
        return; // Timed out mid-read
    }

    // Verify checksum.
    if (frameCheckSum(MAGIC_HOST, &header, payload) != checksum) {
        sendNak(header.addr);
        return;
    }

    switch (header.code) {
    case CODE_READ:
        if (header.len != 1) { // Read commands must be length=1
            sendNak(header.addr);
            break;
        }
        handleRead(&header, payload[0]);
        break;
    case CODE_WRITE:
        handleWrite(&header, payload);
        break;
    default:
        sendNak(header.addr);
        break;
    }
}

/*
** Protocol handling functions
*/

static void sendNak(uint16_t addr)
{
    FrameHeader h = {CODE_NAK, addr, 0};
    sendFrame(&h, NULL);
}

static void handleRead(const FrameHeader *header, uint8_t bytesToRead)
{
    if (bytesToRead > MAX_PAYLOAD) {
        sendNak(header->addr);
        return;
    }

    // Read bytes
    if (!readPage(header->addr, payload, bytesToRead)) {
        sendNak(header->addr);
        return;
    }

    // Send response.
    FrameHeader h = {CODE_ACK, header->addr, bytesToRead};
    sendFrame(&h, payload);
}

static void handleWrite(const FrameHeader *header, const uint8_t *data)
{
    // Write a page.
    if (!writePage(header->addr, data, header->len)) {
        // Couldn't write
        sendNak(header->addr);
        return;
    }

    // Send response.
    FrameHeader h = {CODE_ACK, header->addr, 0};
    sendFrame(&h, NULL);
}

static void sendFrame(const FrameHeader *header, const uint8_t *data)
{
    const uint8_t headerBuf[] = {MAGIC_DEVICE,
                                 header->code,
                                 lowByte(header->addr), // Little Endian encoding.
                                 highByte(header->addr),
                                 header->len};

    // Send header, then payload, and lastly the checksum.
    Serial.write(headerBuf, sizeof(headerBuf));
    if (header->len > 0) {
        Serial.write(data, header->len);
    }
    Serial.write(frameCheckSum(MAGIC_DEVICE, header, data));
}

static uint8_t frameCheckSum(uint8_t magic, const FrameHeader *header, const uint8_t *data)
{
    // XOR is commutative, order doesn't matter.
    uint8_t sum = magic ^ header->code ^ lowByte(header->addr) ^ highByte(header->addr) ^ header->len;
    for (uint8_t i = 0; i < header->len; i++) {
        sum ^= data[i];
    }
    return sum;
}

/*
** Auxiliary and i/o functions
*/

static uint8_t readByte(uint16_t addr)
{
    busToEeprom(addr);
    return readBus();
}

static bool readPage(uint16_t baseAddr, uint8_t *buffer, uint8_t len)
{
    // Check len
    if (len == 0 || len > PAGE_SIZE) {
        return false;
    }
    // Check bounds of address.
    if (baseAddr >= EEPROM_SIZE || len > EEPROM_SIZE - baseAddr) {
        return false;
    }

    for (uint8_t offset = 0; offset < len; offset++) {
        buffer[offset] = readByte(baseAddr + offset);
    }
    return true;
}

static void loadByte(uint16_t addr, uint8_t data)
{
    // Put data out on the buses.
    busToArduino(addr, data);

    // Pulse WE low
    PORTC &= ~_BV(WE_BIT);
    _NOP(); // Ensure t_WP of 100ns.
    _NOP();
    PORTC |= _BV(WE_BIT);
}

static bool writePage(uint16_t baseAddr, const uint8_t *data, uint8_t len)
{
    // Check len
    if (len == 0 || len > PAGE_SIZE) {
        return false;
    }
    // Check address is in bounds.
    if (baseAddr >= EEPROM_SIZE || len > EEPROM_SIZE - baseAddr) {
        return false;
    }
    // Check address is aligned for page writes.
    if (baseAddr % PAGE_SIZE != 0) {
        return false;
    }

    for (uint16_t offset = 0; offset < len; offset++) {
        loadByte(baseAddr + offset, data[offset]);
    }

    return waitForWrite(baseAddr+len-1, data[len-1]);
}

static bool waitForWrite(uint16_t addr, uint8_t expected)
{
    // Wait for t_BLC to expire (150us max)
    delayMicroseconds(200);

    // Write finished when bit 7 contains the correct value.
    uint8_t match = expected & 0x80;
    uint32_t start = millis();
    while (millis() - start < WRITE_TIMEOUT_MS) {
        if ((readByte(addr) & 0x80) == match) {
            return true;            
        }
    }

    // Timed out.
    return false;
}

// Give control of the data bus to the eeprom. Data is available to be read after it's done.
static void busToEeprom(uint16_t addr)
{
    // Release the bus, then give control to the eeprom by setting OE.
    busIn();
    setAddress(addr, true);
    _NOP(); // EEPROM t_ACC + 595 t_PD ~= 200ns. 5 NOPs (62.5ns each) to be safe
    _NOP();
    _NOP();
    _NOP();
    _NOP();
}

// Give control of the data bus to the arduino, write some data to it.
static void busToArduino(uint16_t addr, uint8_t data)
{
    // Disable OE, wait for EEPROM to release bus, then set the data on the bus and
    // set it to output.
    setAddress(addr, false);

    // t_DF is max of 50ns, the time for the EEPROM to release the bus; busWrite takes longer
    // and we set bus to out after it, so no NOP needed.

    // Set data before enabling output.
    busWrite(data);
    busOut();
}

// Set the address and /OE by using the 595 Shift Registers.
// shouldn't be used directly in favor of busToXXX functions.
static void setAddress(uint16_t addr, bool outputEnable)
{
    // OE is active low.
    uint16_t data = (addr & ADDR_MASK) | (outputEnable ? 0x00 : OE_BIT_MASK);

    SPI.beginTransaction(SPISettings(8000000, LSBFIRST, SPI_MODE0));
    SPI.transfer16(data);
    SPI.endTransaction();

    // Toggle latch.
    PORTB |= _BV(SR_LATCH_BIT);
    PORTB &= ~_BV(SR_LATCH_BIT);
}

// Quickly set all the data bus' pins to output.
// shouldn't be used directly in favor of busToXXX functions.
static void busOut(void)
{
    DDRD |= DATA_D_MASK;
    DDRB |= DATA_B_MASK;
}

// Quickly set all the data bus' pins to input.
// shouldn't be used directly in favor of busToXXX functions.
static void busIn(void)
{
    // Set direction.
    DDRD &= ~DATA_D_MASK;
    DDRB &= ~DATA_B_MASK;

    // Clean data in port so it doesn't become pullups.
    PORTD &= ~DATA_D_MASK;
    PORTB &= ~DATA_B_MASK;
}

// Write a byte to the bus.
// shouldn't be used directly in favor of busToXXX functions.
static void busWrite(uint8_t data)
{
    // Clear the bits not on the mask, then set them from data.
    PORTB = (PORTB & ~DATA_B_MASK) | (data >> 6);
    PORTD = (PORTD & ~DATA_D_MASK) | (uint8_t)(data << 2);
}

// Reassemble the 8 bits spread over both ports into a byte.
static uint8_t readBus(void)
{
    return ((PINB & DATA_B_MASK) << 6) | ((PIND & DATA_D_MASK) >> 2);
}
