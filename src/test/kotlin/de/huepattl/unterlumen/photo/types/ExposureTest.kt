package de.huepattl.unterlumen.photo.types

import org.junit.jupiter.api.Assertions.*
import org.junit.jupiter.api.Test

class ExposureTest {

    @Test
    fun `exposure integer`() {
        val oneSecond = ExposureTime.of(1)
        assertEquals("1 sec", oneSecond.toString())
    }

    @Test
    fun `exposure integer minutes`() {
        val oneSecond = ExposureTime.of(1, ExposureTime.Unit.MINUTES)
        assertEquals("1 min", oneSecond.toString())
    }

    @Test
    fun `exposure time fraction`() {
        val oneSecond = ExposureTime.of(1, 250)
        assertEquals("1/250 sec", oneSecond.toString())
    }


}