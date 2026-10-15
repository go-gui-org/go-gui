package com.example.androiddemo

import android.graphics.Rect
import android.os.Bundle
import android.view.View
import android.view.accessibility.AccessibilityEvent
import android.view.accessibility.AccessibilityNodeInfo
import android.view.accessibility.AccessibilityNodeProvider
import androidapp.Androidapp

/**
 * Bridges go-gui A11yNode[] to Android's AccessibilityNodeProvider.
 * Kotlin queries Go for node data via gomobile-exported functions.
 */
class GoGuiAccessibilityProvider(
    private val hostView: View,
    private val density: Float
) : AccessibilityNodeProvider() {

    // AccessRole constants matching gui/shape.go.
    companion object {
        private const val ROLE_NONE = 0
        private const val ROLE_BUTTON = 1
        private const val ROLE_CHECKBOX = 2
        private const val ROLE_COLOR_WELL = 3
        private const val ROLE_COMBO_BOX = 4
        private const val ROLE_DATE_FIELD = 5
        private const val ROLE_DIALOG = 6
        private const val ROLE_DISCLOSURE = 7
        private const val ROLE_GRID = 8
        private const val ROLE_GRID_CELL = 9
        private const val ROLE_GROUP = 10
        private const val ROLE_HEADING = 11
        private const val ROLE_IMAGE = 12
        private const val ROLE_LINK = 13
        private const val ROLE_LIST = 14
        private const val ROLE_LIST_ITEM = 15
        private const val ROLE_MENU = 16
        private const val ROLE_MENU_BAR = 17
        private const val ROLE_MENU_ITEM = 18
        private const val ROLE_PROGRESS_BAR = 19
        private const val ROLE_RADIO_BUTTON = 20
        private const val ROLE_RADIO_GROUP = 21
        private const val ROLE_SCROLL_AREA = 22
        private const val ROLE_SCROLL_BAR = 23
        private const val ROLE_SLIDER = 24
        private const val ROLE_SPLITTER = 25
        private const val ROLE_STATIC_TEXT = 26
        private const val ROLE_SWITCH_TOGGLE = 27
        private const val ROLE_TAB = 28
        private const val ROLE_TAB_ITEM = 29
        private const val ROLE_TEXT_FIELD = 30
        private const val ROLE_TEXT_AREA = 31
        private const val ROLE_TOOLBAR = 32
        private const val ROLE_TREE = 33
        private const val ROLE_TREE_ITEM = 34

        // AccessState bit flags matching gui/shape.go.
        private const val STATE_EXPANDED: Long = 1
        private const val STATE_SELECTED: Long = 2
        private const val STATE_CHECKED: Long = 4
        private const val STATE_DISABLED: Long = 512
    }

    override fun createAccessibilityNodeInfo(
        virtualViewId: Int
    ): AccessibilityNodeInfo? {
        if (virtualViewId == View.NO_ID) {
            // Root node representing the host view.
            return createRootNode()
        }
        val count = Androidapp.a11yNodeCount()
        if (virtualViewId < 0 || virtualViewId >= count) return null

        val node = AccessibilityNodeInfo.obtain(hostView, virtualViewId)
        val role = Androidapp.a11yNodeRole(virtualViewId)
        val label = Androidapp.a11yNodeLabel(virtualViewId)
        val value = Androidapp.a11yNodeValue(virtualViewId)
        val desc = Androidapp.a11yNodeDescription(virtualViewId)
        val state = Androidapp.a11yNodeState(virtualViewId)
        val parentIdx = Androidapp.a11yNodeParent(virtualViewId)

        // Bounds (logical -> physical pixels).
        val x = (Androidapp.a11yNodeBoundsX(virtualViewId) * density).toInt()
        val y = (Androidapp.a11yNodeBoundsY(virtualViewId) * density).toInt()
        val w = (Androidapp.a11yNodeBoundsW(virtualViewId) * density).toInt()
        val h = (Androidapp.a11yNodeBoundsH(virtualViewId) * density).toInt()
        node.setBoundsInParent(Rect(x, y, x + w, y + h))

        // Parent.
        if (parentIdx < 0) {
            node.setParent(hostView)
        } else {
            node.setParent(hostView, parentIdx.toInt())
        }

        // Children.
        val childStart = Androidapp.a11yNodeChildStart(virtualViewId).toInt()
        val childCount = Androidapp.a11yNodeChildCount(virtualViewId).toInt()
        for (i in childStart until childStart + childCount) {
            node.addChild(hostView, i)
        }

        // Content description and text.
        if (label.isNotEmpty()) node.contentDescription = label
        if (value.isNotEmpty()) node.text = value
        if (desc.isNotEmpty() && label.isEmpty()) node.contentDescription = desc

        // Map role to Android class and capabilities.
        applyRole(node, role.toInt())

        // State flags.
        node.isEnabled = (state and STATE_DISABLED) == 0L
        node.isSelected = (state and STATE_SELECTED) != 0L
        node.isChecked = (state and STATE_CHECKED) != 0L

        // Range info for sliders/progress.
        if (role.toInt() == ROLE_SLIDER || role.toInt() == ROLE_PROGRESS_BAR) {
            val vMin = Androidapp.a11yNodeValueMin(virtualViewId)
            val vMax = Androidapp.a11yNodeValueMax(virtualViewId)
            val vNum = Androidapp.a11yNodeValueNum(virtualViewId)
            node.rangeInfo = AccessibilityNodeInfo.RangeInfo.obtain(
                AccessibilityNodeInfo.RangeInfo.RANGE_TYPE_FLOAT,
                vMin, vMax, vNum
            )
        }

        // Focus.
        val focusedIdx = Androidapp.a11yFocusedIndex()
        node.isAccessibilityFocused = (virtualViewId == focusedIdx.toInt())
        node.isVisibleToUser = true

        return node
    }

    override fun performAction(
        virtualViewId: Int,
        action: Int,
        arguments: Bundle?
    ): Boolean {
        if (virtualViewId == View.NO_ID) {
            return hostView.performAccessibilityAction(action, arguments)
        }
        when (action) {
            AccessibilityNodeInfo.ACTION_CLICK -> {
                Androidapp.a11yPerformAction(virtualViewId, 0) // Press
                return true
            }
            AccessibilityNodeInfo.ACTION_SCROLL_FORWARD -> {
                Androidapp.a11yPerformAction(virtualViewId, 1) // Increment
                return true
            }
            AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD -> {
                Androidapp.a11yPerformAction(virtualViewId, 2) // Decrement
                return true
            }
        }
        return false
    }

    private fun createRootNode(): AccessibilityNodeInfo {
        val node = AccessibilityNodeInfo.obtain(hostView)
        node.setSource(hostView)
        node.setParent(hostView.parent as? View)
        node.className = "android.view.View"

        val count = Androidapp.a11yNodeCount()
        // Add top-level children (nodes with parentIdx < 0).
        for (i in 0 until count) {
            val parent = Androidapp.a11yNodeParent(i)
            if (parent < 0) {
                node.addChild(hostView, i)
            }
        }
        return node
    }

    private fun applyRole(node: AccessibilityNodeInfo, role: Int) {
        when (role) {
            ROLE_BUTTON -> {
                node.className = "android.widget.Button"
                node.isClickable = true
                node.addAction(AccessibilityNodeInfo.ACTION_CLICK)
            }
            ROLE_CHECKBOX -> {
                node.className = "android.widget.CheckBox"
                node.isCheckable = true
                node.isClickable = true
                node.addAction(AccessibilityNodeInfo.ACTION_CLICK)
            }
            ROLE_SWITCH_TOGGLE -> {
                node.className = "android.widget.Switch"
                node.isCheckable = true
                node.isClickable = true
                node.addAction(AccessibilityNodeInfo.ACTION_CLICK)
            }
            ROLE_SLIDER -> {
                node.className = "android.widget.SeekBar"
                node.addAction(AccessibilityNodeInfo.ACTION_SCROLL_FORWARD)
                node.addAction(AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD)
            }
            ROLE_PROGRESS_BAR -> {
                node.className = "android.widget.ProgressBar"
            }
            ROLE_TEXT_FIELD, ROLE_TEXT_AREA -> {
                node.className = "android.widget.EditText"
                node.isEditable = true
                node.isClickable = true
                node.addAction(AccessibilityNodeInfo.ACTION_CLICK)
            }
            ROLE_IMAGE -> {
                node.className = "android.widget.ImageView"
            }
            ROLE_STATIC_TEXT, ROLE_HEADING -> {
                node.className = "android.widget.TextView"
            }
            ROLE_LINK -> {
                node.className = "android.widget.TextView"
                node.isClickable = true
                node.addAction(AccessibilityNodeInfo.ACTION_CLICK)
            }
            ROLE_MENU_ITEM, ROLE_TAB_ITEM, ROLE_LIST_ITEM,
            ROLE_TREE_ITEM, ROLE_RADIO_BUTTON, ROLE_DISCLOSURE -> {
                node.className = "android.view.View"
                node.isClickable = true
                node.addAction(AccessibilityNodeInfo.ACTION_CLICK)
            }
            ROLE_SCROLL_AREA -> {
                node.className = "android.widget.ScrollView"
                node.isScrollable = true
            }
            else -> {
                node.className = "android.view.View"
            }
        }
    }
}
