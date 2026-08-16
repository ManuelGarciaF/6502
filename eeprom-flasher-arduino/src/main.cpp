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

// Static verification.
static_assert((ADDR_MASK & OE_BIT_MASK) == 0, "/OE must not overlap with the address");
static_assert((ADDR_MASK | OE_BIT_MASK) == 0xFFFF, "All SR outputs must be accounted for");

static_assert(__builtin_popcount(DATA_D_MASK) + __builtin_popcount(DATA_B_MASK) == 8, "Data bus must cover 8 pins");

static_assert((DATA_B_MASK & _BV(SR_LATCH_BIT)) == 0, "Data bus must not overlap SR latch");
static_assert((DATA_B_MASK & _BV(PB3)) == 0, "Data bus must not overlap MOSI");
static_assert((DATA_B_MASK & _BV(PB5)) == 0, "Data bus must not overlap SCK");

/*
** Function definitions
*/
static void dumpContents();
static void busToEeprom(uint16_t addr);
static void busToArduino(uint16_t addr, uint8_t data);
static void setAddress(uint16_t addr, bool outputEnable);
static uint8_t readByte(uint16_t addr);
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
    SPI.begin();

    PORTB &= ~_BV(SR_LATCH_BIT); // Set latch low just in case.

    /* END OF SETUP */
    dumpContents();
}

void loop(void)
{
}

/*
** Auxiliary functions
*/

static void dumpContents()
{
    for (uint16_t addr = 0x0000; addr < 0x8000; addr += 8) {

        uint8_t page[64];
        for (uint16_t offset = 0; offset < sizeof(page); offset++) {
            page[offset] = readByte(addr + offset);
        }

        Serial.write(page, sizeof(page));
    }

    // Return the bus to a known state.
    busToArduino(0x0000, 0x00);
}

// Give control of the data bus to the eeprom. Data is available to be read after its done.
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

    // t_DF is max of 50ns, the time for the EEPROM to release the bus, busWrite takes longer
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

static uint8_t readByte(uint16_t addr)
{
    busToEeprom(addr);
    return readBus();
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
