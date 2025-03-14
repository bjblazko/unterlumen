package de.huepattl.unterlumen.photo.types

import org.junit.jupiter.api.Assertions.*
import org.junit.jupiter.api.Test

class QualityTest {

    @Test
    fun `colour depth bits to string`() {
        val precision = ColourDepth(value = 8, unit = ColourDepth.Unit.BIT)
        assertEquals("8 bit", precision.toString())
    }

    @Test
    fun `colour depth bits create from string`() {
        val precision = ColourDepth.of("8 bit")
        assertEquals(8, precision.value)
        assertEquals(ColourDepth.Unit.BIT, precision.unit)
    }

}